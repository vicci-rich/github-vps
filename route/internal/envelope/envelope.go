// Package envelope turns a git commit into the signed <envelope> document and
// verifies that document against a pinned keyring.
package envelope

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"route/internal/gitx"
	"route/internal/gpgx"
	"route/internal/xmlio"
)

// Builder produces envelopes for one repository.
type Builder struct {
	Repo   string
	Outbox string
}

// Build reads the commit at rev ("" or "HEAD" allowed) and returns the
// envelope document, validated against route.xsd.
func (b *Builder) Build(rev string) (*xmlio.Envelope, error) {
	sha, err := gitx.Head(b.Repo, rev)
	if err != nil {
		return nil, err
	}
	c, err := gitx.ReadCommit(b.Repo, sha)
	if err != nil {
		return nil, err
	}
	files, ins, del, err := gitx.DiffStat(b.Repo, sha)
	if err != nil {
		return nil, err
	}
	digest, err := gitx.GraphDigest(b.Repo)
	if err != nil {
		return nil, err
	}
	fpr, ok, err := gitx.VerifyCommit(b.Repo, sha)
	if err != nil {
		return nil, err
	}
	env := &xmlio.Envelope{
		Sha:         c.Sha,
		Author:      c.AuthorEmail,
		Fpr:         fpr,
		Tree:        c.Tree,
		Parents:     strings.Join(c.Parents, ","),
		CommittedAt: c.CommittedAt.Format(time.RFC3339),
		Verified:    boolString(ok),
		Sig:         c.Sig,
		Diffstat:    &xmlio.Diffstat{Files: files, Insertions: ins, Deletions: del},
		GraphDigest: digest,
	}
	if err := xmlio.Validate(env); err != nil {
		return nil, err
	}
	return env, nil
}

// Canonical is the payload a signer signs when the envelope is produced
// independently of git (detached-signature mode). The git path signs the
// commit object itself; this is the same bytes without the signature.
func Canonical(env *xmlio.Envelope) ([]byte, error) {
	type canon struct {
		Sha         string `json:"sha"`
		Author      string `json:"author"`
		Fpr         string `json:"fpr,omitempty"`
		Tree        string `json:"tree,omitempty"`
		Parents     string `json:"parents,omitempty"`
		CommittedAt string `json:"committedAt"`
		Verified    string `json:"verified"`
		Files       int    `json:"files"`
		Insertions  int    `json:"insertions"`
		Deletions   int    `json:"deletions"`
		GraphDigest string `json:"graphDigest,omitempty"`
	}
	c := canon{
		Sha: env.Sha, Author: env.Author, Fpr: env.Fpr, Tree: env.Tree,
		Parents: env.Parents, CommittedAt: env.CommittedAt, Verified: env.Verified,
		GraphDigest: env.GraphDigest,
	}
	if env.Diffstat != nil {
		c.Files, c.Insertions, c.Deletions = env.Diffstat.Files, env.Diffstat.Insertions, env.Diffstat.Deletions
	}
	return json.Marshal(c)
}

// Write stores the envelope as <outbox>/<sha>.xml and returns the path.
func Write(outbox string, env *xmlio.Envelope) (string, error) {
	if err := os.MkdirAll(outbox, 0o755); err != nil {
		return "", err
	}
	body, err := xmlio.Marshal(env)
	if err != nil {
		return "", err
	}
	path := filepath.Join(outbox, env.Sha+".xml")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Verify checks the commit signature against a pinned keyring directory
// (files matching *.asc). An empty keysDir falls back to the local keyring
// through `git verify-commit`.
func Verify(h *gpgx.Home, repo, sha, keysDir string) (fpr string, ok bool, err error) {
	if keysDir == "" {
		return gitx.VerifyCommit(repo, sha)
	}
	keys, err := filepath.Glob(filepath.Join(keysDir, "*.asc"))
	if err != nil {
		return "", false, err
	}
	if len(keys) == 0 {
		return "", false, fmt.Errorf("no pinned keys in %s", keysDir)
	}
	for _, path := range keys {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", false, readErr
		}
		if importErr := h.Import(string(body)); importErr != nil {
			return "", false, importErr
		}
	}
	c, err := gitx.ReadCommit(repo, sha)
	if err != nil {
		return "", false, err
	}
	if !c.Signed {
		return "", false, nil
	}
	return h.Verify(c.Payload(), c.Sig)
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
