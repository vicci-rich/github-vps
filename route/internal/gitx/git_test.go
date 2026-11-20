package gitx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"user.name": "Test", "user.email": "t@example.com"} {
		if err := Config(dir, k, v); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func commitFile(t *testing.T, dir, name, body, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, "add", name); err != nil {
		t.Fatal(err)
	}
	sha, err := CommitStaged(dir, msg, false, "")
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func TestReadCommitAndDiffStat(t *testing.T) {
	dir := newRepo(t)
	sha := commitFile(t, dir, "a.txt", "hello\n", "first")

	c, err := ReadCommit(dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if c.AuthorEmail != "t@example.com" || c.AuthorName != "Test" {
		t.Fatalf("author = %q <%q>", c.AuthorName, c.AuthorEmail)
	}
	if c.Tree == "" || len(c.Parents) != 0 || c.Message != "first" {
		t.Fatalf("unexpected commit: %+v", c)
	}
	if c.Signed || c.Sig != "" {
		t.Fatalf("unsigned commit reported a signature")
	}
	payload := string(c.Payload())
	if !strings.HasPrefix(payload, "tree "+c.Tree) || !strings.HasSuffix(payload, "first\n") {
		t.Fatalf("payload does not reconstruct the commit: %q", payload)
	}

	files, ins, del, err := DiffStat(dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if files != 1 || ins != 1 || del != 0 {
		t.Fatalf("diffstat = %d/%d/%d, want 1/1/0", files, ins, del)
	}

	digest, err := GraphDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 64 {
		t.Fatalf("graph digest length = %d", len(digest))
	}

	fpr, ok, err := VerifyCommit(dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if ok || fpr != "" {
		t.Fatalf("unsigned commit verified: fpr=%q ok=%t", fpr, ok)
	}
}

func TestSecondCommitHasParent(t *testing.T) {
	dir := newRepo(t)
	first := commitFile(t, dir, "a.txt", "hello\n", "first")
	second := commitFile(t, dir, "b.txt", "world\n", "second")

	c, err := ReadCommit(dir, second)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Parents) != 1 || c.Parents[0] != first {
		t.Fatalf("parents = %v, want [%s]", c.Parents, first)
	}
}

func TestAddIntentAndAmend(t *testing.T) {
	dir := newRepo(t)
	sha := commitFile(t, dir, "a.txt", "hello\n", "first")

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddIntent(dir, "new.txt"); err != nil {
		t.Fatalf("AddIntent: %v", err)
	}
	status, err := Run(dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "new.txt") {
		t.Fatalf("intent-to-add not recorded: %q", status)
	}

	amended, err := Amend(dir, "first, edited", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if amended == sha {
		t.Fatal("amend did not rewrite the commit")
	}
	c, err := ReadCommit(dir, amended)
	if err != nil {
		t.Fatal(err)
	}
	if c.Message != "first, edited" {
		t.Fatalf("message = %q", c.Message)
	}
}

func TestErrorPaths(t *testing.T) {
	if _, err := Head(t.TempDir(), "HEAD"); err == nil {
		t.Fatal("expected an error from rev-parse outside a repository")
	}
	if _, err := Run(t.TempDir(), "not-a-command"); err == nil {
		t.Fatal("expected an error from an unknown git command")
	}
}
