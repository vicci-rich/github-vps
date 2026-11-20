package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"route/internal/gitx"
	"route/internal/xmlio"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := gitx.Init(dir); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"user.name": "Test", "user.email": "t@example.com"} {
		if err := gitx.Config(dir, k, v); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPutThenEnvelopeThenVerify(t *testing.T) {
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "record.txt"), []byte("row\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outbox := filepath.Join(t.TempDir(), "outbox")

	if err := run([]string{"put", "-C", repo, "-o", outbox, "-S=false", "-m", "cli put", "record.txt"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	entries, err := os.ReadDir(outbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("outbox has %d entries, want 1", len(entries))
	}
	path := filepath.Join(outbox, entries[0].Name())
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := xmlio.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	env, ok := doc.(*xmlio.Envelope)
	if !ok || env.Verified != "false" {
		t.Fatalf("unexpected envelope %#v", doc)
	}

	// An unsigned commit must not verify, and the CLI must say so.
	err = run([]string{"verify", "-C", repo, "-k", filepath.Join(t.TempDir(), "missing"), path})
	if err == nil || !strings.Contains(err.Error(), "did not verify") {
		t.Fatalf("verify unsigned = %v, want a verification failure", err)
	}

	if err := run([]string{"get", "-C", repo}); err != nil {
		t.Fatalf("get: %v", err)
	}
}

func TestAmendAndRm(t *testing.T) {
	repo := initRepo(t)
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outbox := filepath.Join(t.TempDir(), "outbox")
	if err := run([]string{"put", "-C", repo, "-o", outbox, "-S=false", "-m", "two files", "a.txt", "b.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"amend", "-C", repo, "-o", outbox, "-S=false", "-m", "two files, edited"}); err != nil {
		t.Fatalf("amend: %v", err)
	}
	if err := run([]string{"rm", "-C", repo, "-o", outbox, "-S=false", "-m", "drop b", "b.txt"}); err != nil {
		t.Fatalf("rm: %v", err)
	}
	entries, err := os.ReadDir(outbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("outbox has %d entries, want 3 (put, amend, rm)", len(entries))
	}
}

func TestInstallHooks(t *testing.T) {
	repo := initRepo(t)
	hooks := filepath.Join(t.TempDir(), "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(hooks, "post-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"install-hooks", "-C", repo, "-H", hooks}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("hook was not made executable")
	}
	got, err := gitx.Run(repo, "config", "--get", "core.hooksPath")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != hooks {
		t.Fatalf("core.hooksPath = %q, want %q", strings.TrimSpace(got), hooks)
	}
}

func TestAuxiliaryCommands(t *testing.T) {
	if err := run([]string{"version"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"help"}); err != nil {
		t.Fatal(err)
	}
	if err := run(nil); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"frobnicate"}); err == nil {
		t.Fatal("expected an error for an unknown command")
	}
	if err := run([]string{"verify"}); err == nil {
		t.Fatal("expected an error for a missing file argument")
	}
	if err := run([]string{"verify", filepath.Join(t.TempDir(), "missing.xml")}); err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if err := run([]string{"put", "-C", t.TempDir()}); err == nil {
		t.Fatal("expected an error when no path is given")
	}
}

func TestWatchAndSchemaCommands(t *testing.T) {
	feed := filepath.Join(t.TempDir(), "feed.jsonl")
	body := `{"kind":"block","height":5,"hash":"h","parent_hash":"p","timestamp":1,"tx_count":1}
{"kind":"tx","height":5,"txid":"t","from":"a","to":"b","value":"1","status":"ok"}
{"kind":"omni","height":5,"txid":"t","property_id":31,"amount":"2.5","valid":true}
`
	if err := os.WriteFile(feed, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"watch", "-f", feed, "-chain", "justitia"}); err != nil {
		t.Fatalf("watch: %v", err)
	}
	if err := run([]string{"watch", "-f", filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected an error for a missing feed file")
	}
	if err := run([]string{"schema"}); err != nil {
		t.Fatalf("schema: %v", err)
	}
}

func TestArtifactsCommandValidation(t *testing.T) {
	if err := run([]string{"artifacts"}); err == nil {
		t.Fatal("expected an error without a verb")
	}
	if err := run([]string{"artifacts", "frobnicate"}); err == nil {
		t.Fatal("expected an error for an unknown verb")
	}
	if err := run([]string{"artifacts", "put"}); err == nil {
		t.Fatal("expected an error without -f/-name")
	}
	t.Setenv("ROUTE_S3_ACCESS_KEY", "")
	t.Setenv("ROUTE_S3_SECRET_KEY", "")
	file := filepath.Join(t.TempDir(), "a.json")
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"artifacts", "put", "-f", file, "-name", "abi/a.json"}); err == nil {
		t.Fatal("expected an error without credentials")
	}
}
