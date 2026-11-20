package proof

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"route/internal/envelope"
	"route/internal/gitx"
	"route/internal/gpgx"
)

func signedRepo(t *testing.T) (repo, sha, fpr, pub string) {
	t.Helper()
	repo = t.TempDir()
	if err := gitx.Init(repo); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"user.name": "Proof", "user.email": "proof@example.com"} {
		if err := gitx.Config(repo, k, v); err != nil {
			t.Fatal(err)
		}
	}
	home, err := gpgx.NewHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	fpr, err = home.GenerateKey("Proof", "proof@example.com")
	if err != nil {
		t.Fatal(err)
	}
	pub, err = home.Export(fpr)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GNUPGHOME", home.Dir)
	if err := gitx.Config(repo, "user.signingkey", fpr); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "row.txt"), []byte("row\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(repo, "add", "row.txt"); err != nil {
		t.Fatal(err)
	}
	sha, err = gitx.CommitStaged(repo, "proof row", true, "")
	if err != nil {
		t.Fatal(err)
	}
	return repo, sha, fpr, pub
}

func storeFor(t *testing.T, repo, sha, pub string) (*Store, string) {
	t.Helper()
	outbox := filepath.Join(t.TempDir(), "outbox")
	env, err := (&envelope.Builder{Repo: repo, Outbox: outbox}).Build(sha)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := envelope.Write(outbox, env); err != nil {
		t.Fatal(err)
	}
	keys := filepath.Join(t.TempDir(), "keys")
	if pub != "" {
		if err := os.MkdirAll(keys, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(keys, "proof.asc"), []byte(pub), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Store{Repo: repo, Outbox: outbox, Keys: keys, Receipts: map[string]Receipt{}}, sha
}

func TestBuildVerifiesGPGAndReportsNoReceipt(t *testing.T) {
	repo, sha, fpr, pub := signedRepo(t)
	s, _ := storeFor(t, repo, sha, pub)

	p, err := s.Build(sha)
	if err != nil {
		t.Fatal(err)
	}
	if p.GPG.Verified != "true" || p.GPG.Fpr != fpr {
		t.Fatalf("gpg = %+v, want true/%s", p.GPG, fpr)
	}
	if p.Receipt.Verified != "not-recomputed" {
		t.Fatalf("receipt verified = %q, want not-recomputed", p.Receipt.Verified)
	}
}

func TestBuildIncludesReceiptEvidence(t *testing.T) {
	repo, sha, _, pub := signedRepo(t)
	s, _ := storeFor(t, repo, sha, pub)
	s.Receipts[sha] = Receipt{Root: "aa", TxIndex: 3, HeaderSigData: "bb", Verified: "true"}

	p, err := s.Build(sha)
	if err != nil {
		t.Fatal(err)
	}
	if p.Receipt.Verified != "true" || p.Receipt.Root != "aa" || *p.Receipt.TxIndex != 3 {
		t.Fatalf("receipt = %+v", p.Receipt)
	}

	s.Receipts[sha] = Receipt{Verified: "trust-me"}
	p, err = s.Build(sha)
	if err != nil {
		t.Fatal(err)
	}
	if p.Receipt.Verified != "not-recomputed" {
		t.Fatalf("untrusted receipt value accepted: %q", p.Receipt.Verified)
	}
}

func TestBuildWithoutRepoDoesNotClaimGPG(t *testing.T) {
	repo, sha, _, pub := signedRepo(t)
	s, _ := storeFor(t, repo, sha, pub)
	s.Repo = ""

	p, err := s.Build(sha)
	if err != nil {
		t.Fatal(err)
	}
	if p.GPG.Verified != "not-recomputed" {
		t.Fatalf("gpg verified = %q without a repo", p.GPG.Verified)
	}
}

func TestBuildErrors(t *testing.T) {
	s := &Store{Outbox: t.TempDir(), Receipts: map[string]Receipt{}}
	if _, err := s.Build("../etc/passwd"); err == nil {
		t.Fatal("expected an error for a non-sha")
	}
	if _, err := s.Build("abcdef0"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing envelope: %v", err)
	}
}

func TestHandler(t *testing.T) {
	repo, sha, _, pub := signedRepo(t)
	s, _ := storeFor(t, repo, sha, pub)
	srv := httptest.NewServer(Handler(s))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/proof/" + sha)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Fatalf("content type = %q", ct)
	}
	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	if !strings.Contains(string(buf[:n]), "<proof") {
		t.Fatalf("body does not start a proof: %q", buf[:n])
	}

	for _, tc := range []struct {
		path   string
		method string
		want   int
	}{
		{"/proof/ZZZ", http.MethodGet, http.StatusBadRequest},
		{"/proof/abcdef0", http.MethodGet, http.StatusNotFound},
		{"/proof/" + sha, http.MethodPost, http.StatusMethodNotAllowed},
	} {
		req, err := http.NewRequest(tc.method, srv.URL+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, resp.StatusCode, tc.want)
		}
	}
}
