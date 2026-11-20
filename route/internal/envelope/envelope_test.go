package envelope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"route/internal/gitx"
	"route/internal/gpgx"
	"route/internal/xmlio"
)

func repoWithCommit(t *testing.T, signed bool) (dir, sha, fpr, pub string) {
	t.Helper()
	dir = t.TempDir()
	if err := gitx.Init(dir); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"user.name": "Test", "user.email": "t@example.com"} {
		if err := gitx.Config(dir, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if signed {
		home, err := gpgx.NewHome(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = home.Close() })
		fpr, err = home.GenerateKey("Test", "t@example.com")
		if err != nil {
			t.Fatal(err)
		}
		pub, err = home.Export(fpr)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("GNUPGHOME", home.Dir)
		if err := gitx.Config(dir, "user.signingkey", fpr); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(dir, "add", "a.txt"); err != nil {
		t.Fatal(err)
	}
	var err error
	sha, err = gitx.CommitStaged(dir, "route: first record", signed, "")
	if err != nil {
		t.Fatal(err)
	}
	return dir, sha, fpr, pub
}

func TestBuildUnsigned(t *testing.T) {
	dir, sha, _, _ := repoWithCommit(t, false)
	outbox := filepath.Join(t.TempDir(), "outbox")
	b := &Builder{Repo: dir, Outbox: outbox}
	env, err := b.Build("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if env.Sha != sha || env.Verified != "false" {
		t.Fatalf("sha=%s verified=%s, want %s/false", env.Sha, env.Verified, sha)
	}
	if env.Author != "t@example.com" || env.Diffstat == nil || env.Diffstat.Files != 1 {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	path, err := Write(outbox, env)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := xmlio.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := doc.(*xmlio.Envelope)
	if !ok || got.Sha != sha {
		t.Fatalf("parsed back %#v", doc)
	}
}

func TestBuildSignedAndVerifyWithPinnedKeyring(t *testing.T) {
	dir, sha, fpr, pub := repoWithCommit(t, true)

	b := &Builder{Repo: dir, Outbox: filepath.Join(t.TempDir(), "outbox")}
	env, err := b.Build("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if env.Verified != "true" || env.Fpr != fpr {
		t.Fatalf("verified=%s fpr=%s, want true/%s", env.Verified, env.Fpr, fpr)
	}

	keys := filepath.Join(t.TempDir(), "keys")
	if err := os.MkdirAll(keys, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keys, fpr+".asc"), []byte(pub), 0o644); err != nil {
		t.Fatal(err)
	}

	verifier, err := gpgx.NewHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer verifier.Close()
	got, ok, err := Verify(verifier, dir, sha, keys)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != fpr {
		t.Fatalf("pinned verify = (%q, %t), want (%q, true)", got, ok, fpr)
	}
}

func TestCanonicalOmitsSignature(t *testing.T) {
	env := &xmlio.Envelope{
		Sha: "9f2c1d7b3a4e5f60718293a4b5c6d7e8f9012345", Author: "a@b.c",
		CommittedAt: "2026-11-20T10:00:00Z", Verified: "true", Sig: "SECRET",
		Diffstat: &xmlio.Diffstat{Files: 1, Insertions: 2, Deletions: 3},
	}
	body, err := Canonical(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || strings.Contains(string(body), "SECRET") {
		t.Fatalf("canonical payload must not carry the signature: %s", body)
	}
}

func TestVerifyWithoutPinnedKeys(t *testing.T) {
	dir, sha, _, _ := repoWithCommit(t, false)
	home, err := gpgx.NewHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	empty := filepath.Join(t.TempDir(), "keys")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Verify(home, dir, sha, empty); err == nil || ok {
		t.Fatalf("expected an error when the keyring is empty, got ok=%t err=%v", ok, err)
	}
	if _, ok, err := Verify(home, dir, sha, ""); err != nil || ok {
		t.Fatalf("unsigned with local keyring: ok=%t err=%v", ok, err)
	}
}
