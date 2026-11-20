// Package proof assembles the <proof> document served at /proof/{sha}: the
// envelope stored by the backend, the GPG verdict for its commit, and the
// receipt status reported honestly (never a bare "true" when nothing was
// recomputed).
package proof

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"route/internal/envelope"
	"route/internal/gpgx"
	"route/internal/xmlio"
)

// Receipt is what the light node knows about a transaction's receipt.
type Receipt struct {
	Root          string `json:"root"`
	TxIndex       int    `json:"tx_index"`
	HeaderSigData string `json:"header_sig_data"`
	Verified      string `json:"verified"`
}

// Store holds the locations and the optional receipt map.
type Store struct {
	Repo     string             // git repository; empty means "cannot recompute GPG here"
	Outbox   string             // directory with <sha>.xml
	Keys     string             // pinned keyring (*.asc); empty falls back to git
	Receipts map[string]Receipt // sha -> receipt evidence
}

// ErrNotFound is returned when no envelope exists for the sha.
var ErrNotFound = errors.New("proof: envelope not found")

var shaRE = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// LoadReceipts reads the optional receipts JSON file.
func LoadReceipts(path string) (map[string]Receipt, error) {
	if path == "" {
		return map[string]Receipt{}, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]Receipt
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("proof: %s: %w", path, err)
	}
	return out, nil
}

// Build assembles the proof for a sha.
func (s *Store) Build(sha string) (*xmlio.Proof, error) {
	if !shaRE.MatchString(sha) {
		return nil, fmt.Errorf("proof: %q is not a commit sha", sha)
	}
	path := filepath.Join(s.Outbox, sha+".xml")
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	doc, err := xmlio.Parse(body)
	if err != nil {
		return nil, err
	}
	if err := xmlio.Validate(doc); err != nil {
		return nil, err
	}
	env, ok := doc.(*xmlio.Envelope)
	if !ok {
		return nil, fmt.Errorf("proof: %s is not an envelope", path)
	}

	p := &xmlio.Proof{Sha: env.Sha}
	p.GPG = &xmlio.GPGProof{Verified: "not-recomputed"}
	if s.Repo != "" {
		home, err := gpgx.NewHome("")
		if err != nil {
			return nil, err
		}
		defer home.Close()
		fpr, good, err := envelope.Verify(home, s.Repo, env.Sha, s.Keys)
		if err != nil {
			return nil, err
		}
		p.GPG.Fpr = fpr
		p.GPG.Verified = boolText(good)
	}

	p.Receipt = &xmlio.ReceiptProof{Verified: "not-recomputed"}
	if r, ok := s.Receipts[env.Sha]; ok {
		idx := r.TxIndex
		p.Receipt = &xmlio.ReceiptProof{
			Root:          r.Root,
			TxIndex:       &idx,
			HeaderSigData: r.HeaderSigData,
			Verified:      normalise(r.Verified),
		}
	}
	if err := xmlio.Validate(p); err != nil {
		return nil, err
	}
	return p, nil
}

func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func normalise(v string) string {
	switch v {
	case "true", "false", "not-recomputed":
		return v
	default:
		return "not-recomputed"
	}
}
