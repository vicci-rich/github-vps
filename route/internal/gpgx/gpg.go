// Package gpgx wraps the GnuPG calls the backend needs: generating an
// isolated test key, detached-signing the canonical envelope, verifying a
// detached signature against a pinned keyring, and importing/exporting keys.
package gpgx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Home is an isolated GNUPGHOME.
type Home struct{ Dir string }

// NewHome creates a private keyring directory (0700) for tests or for a
// verifier that must not read the user's personal keyring.
func NewHome(parent string) (*Home, error) {
	dir, err := os.MkdirTemp(parent, "route-gnupg-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	return &Home{Dir: dir}, nil
}

// Close removes the keyring directory.
func (h *Home) Close() error { return os.RemoveAll(h.Dir) }

func (h *Home) run(args ...string) (string, string, error) {
	cmd := exec.Command("gpg", args...)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+h.Dir)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

// GenerateKey creates a signing key without a passphrase and returns its
// fingerprint.
func (h *Home) GenerateKey(name, email string) (string, error) {
	uid := fmt.Sprintf("%s <%s>", name, email)
	if _, errs, err := h.run("--batch", "--passphrase", "", "--quick-generate-key", uid, "ed25519", "sign", "0"); err != nil {
		return "", fmt.Errorf("gpg generate: %w: %s", err, strings.TrimSpace(errs))
	}
	out, errs, err := h.run("--batch", "--with-colons", "--list-secret-keys", email)
	if err != nil {
		return "", fmt.Errorf("gpg list: %w: %s", err, strings.TrimSpace(errs))
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "fpr:") {
			fields := strings.Split(line, ":")
			if len(fields) > 9 {
				return fields[9], nil
			}
		}
	}
	return "", fmt.Errorf("gpg list: no fingerprint for %s", email)
}

// Sign returns an armored detached signature over data.
func (h *Home) Sign(data []byte, fpr string) (string, error) {
	dir, err := os.MkdirTemp("", "route-sign-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "payload")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return "", err
	}
	args := []string{"--batch", "--armor", "--detach-sign", "--output", "-"}
	if fpr != "" {
		args = append(args, "--local-user", fpr)
	}
	args = append(args, in)
	out, errs, runErr := h.run(args...)
	if runErr != nil {
		return "", fmt.Errorf("gpg sign: %w: %s", runErr, strings.TrimSpace(errs))
	}
	return out, nil
}

// Verify checks an armored detached signature against data and returns the
// signer fingerprint. A bad signature yields ok=false with no error.
func (h *Home) Verify(data []byte, armor string) (fpr string, ok bool, err error) {
	dir, err := os.MkdirTemp("", "route-verify-")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(dir)
	payload := filepath.Join(dir, "payload")
	sig := filepath.Join(dir, "payload.asc")
	if err := os.WriteFile(payload, data, 0o600); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(sig, []byte(armor), 0o600); err != nil {
		return "", false, err
	}
	out, errs, runErr := h.run("--batch", "--status-fd", "1", "--verify", sig, payload)
	for _, line := range strings.Split(out+"\n"+errs, "\n") {
		fields := strings.Fields(line)
		for i, tok := range fields {
			if tok == "VALIDSIG" && i+1 < len(fields) {
				fpr = fields[i+1]
			}
		}
	}
	if runErr != nil || fpr == "" {
		return "", false, nil
	}
	return fpr, true, nil
}

// Export returns the armored public key for fpr.
func (h *Home) Export(fpr string) (string, error) {
	out, errs, err := h.run("--batch", "--armor", "--export", fpr)
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("gpg export %s: %v: %s", fpr, err, strings.TrimSpace(errs))
	}
	return out, nil
}

// Import loads an armored public key into the keyring.
func (h *Home) Import(armor string) error {
	dir, err := os.MkdirTemp("", "route-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "key.asc")
	if err := os.WriteFile(file, []byte(armor), 0o600); err != nil {
		return err
	}
	if _, errs, err := h.run("--batch", "--import", file); err != nil {
		return fmt.Errorf("gpg import: %w: %s", err, strings.TrimSpace(errs))
	}
	return nil
}
