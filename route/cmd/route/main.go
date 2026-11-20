// Command route is the git-backed CRUD front end: it turns commits into
// signed <envelope> documents, verifies them against a pinned keyring, and
// exposes the same put/get/amend/rm verbs to the daemon and the web app.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"route/internal/envelope"
	"route/internal/gitx"
	"route/internal/gpgx"
	"route/internal/xmlio"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "route:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "envelope":
		return cmdEnvelope(rest)
	case "verify":
		return cmdVerify(rest)
	case "get":
		return cmdGet(rest)
	case "put":
		return cmdPut(rest)
	case "amend":
		return cmdAmend(rest)
	case "rm":
		return cmdRm(rest)
	case "install-hooks":
		return cmdInstallHooks(rest)
	case "watch":
		return cmdWatch(rest)
	case "schema":
		return cmdSchema(rest)
	case "ticker":
		return cmdTicker(rest)
	case "proof":
		return cmdProof(rest)
	case "serve":
		return cmdServe(rest)
	case "artifacts":
		return cmdArtifacts(rest)
	case "audit":
		return cmdAudit(rest)
	case "chaininfo":
		return cmdChaininfo(rest)
	case "version", "-v", "--version":
		fmt.Println("route", version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	}
	return fmt.Errorf("unknown command %q", cmd)
}

func usage() {
	fmt.Fprint(os.Stderr, `route - git-backed CRUD front end

usage:
  route envelope [rev]        build outbox/<sha>.xml for a commit (default HEAD)
  route verify <file>         validate the XML and verify the signature
  route get <rev>             print the envelope for a commit
  route put [-m msg] <paths>  git add -N, commit, then write the envelope
  route amend [-m msg]        rewrite the last commit, then write the envelope
  route rm <paths>            git rm, commit, then write the envelope
  route install-hooks [-H d]  set core.hooksPath to the hooks directory
  route watch -f feed.jsonl   apply a recorded feed to the store (stdout JSON)
  route schema                print the Postgres DDL
  route ticker -f markets.json serve /markets.json and /assets.json
  route proof [-o d] [-k d] <sha>  print the <proof> document (flags first)
  route serve [-o d] [-k d] [-receipts f] [-addr a]  serve GET /proof/{sha}
  route artifacts put|get ...  ABI/metadata registry (S3-compatible)
  route audit [-f inventory.json]  check the duplication policy
  route chaininfo [-rpc url] [-address 0x..]  second caller (JSON-RPC)
  route version               print the version

flags (per command): -C repo  -o outbox  -k keys
`)
}

func newSet(name string) (*flag.FlagSet, *string, *string, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repo := fs.String("C", ".", "repository")
	outbox := fs.String("o", "outbox", "outbox directory")
	keys := fs.String("k", "keys", "pinned keyring directory (*.asc)")
	return fs, repo, outbox, keys
}

func cmdEnvelope(args []string) error {
	fs, repo, outbox, _ := newSet("envelope")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rev := "HEAD"
	if fs.NArg() > 0 {
		rev = fs.Arg(0)
	}
	b := &envelope.Builder{Repo: *repo, Outbox: *outbox}
	env, err := b.Build(rev)
	if err != nil {
		return err
	}
	path, err := envelope.Write(*outbox, env)
	if err != nil {
		return err
	}
	fmt.Printf("%s verified=%s", path, env.Verified)
	if env.Fpr != "" {
		fmt.Printf(" fpr=%s", env.Fpr)
	}
	fmt.Println()
	return nil
}

func cmdVerify(args []string) error {
	fs, repo, _, keys := newSet("verify")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("verify: expected one file")
	}
	body, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	doc, err := xmlio.Parse(body)
	if err != nil {
		return err
	}
	if err := xmlio.Validate(doc); err != nil {
		return err
	}
	env, ok := doc.(*xmlio.Envelope)
	if !ok {
		return fmt.Errorf("verify: %s is not an envelope", fs.Arg(0))
	}
	home, err := gpgx.NewHome("")
	if err != nil {
		return err
	}
	defer home.Close()
	keydir := *keys
	if _, statErr := os.Stat(keydir); statErr != nil {
		keydir = ""
	}
	fpr, good, err := envelope.Verify(home, *repo, env.Sha, keydir)
	if err != nil {
		return err
	}
	fmt.Printf("%s verified=%t", env.Sha, good)
	if fpr != "" {
		fmt.Printf(" fpr=%s", fpr)
	}
	fmt.Println()
	if !good {
		return fmt.Errorf("verify: signature for %s did not verify", env.Sha)
	}
	return nil
}

func cmdGet(args []string) error {
	fs, repo, _, _ := newSet("get")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rev := "HEAD"
	if fs.NArg() > 0 {
		rev = fs.Arg(0)
	}
	b := &envelope.Builder{Repo: *repo}
	env, err := b.Build(rev)
	if err != nil {
		return err
	}
	body, err := xmlio.Marshal(env)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(body)
	return err
}

func cmdPut(args []string) error {
	fs, repo, outbox, _ := newSet("put")
	msg := fs.String("m", "route: put", "commit message")
	sign := fs.Bool("S", true, "sign the commit (git commit -S)")
	program := fs.String("g", "", "gpg program for signing (default: repository config)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("put: expected at least one path")
	}
	if err := gitx.Add(*repo, fs.Args()...); err != nil {
		return err
	}
	sha, err := gitx.CommitStaged(*repo, *msg, *sign, *program)
	if err != nil {
		return err
	}
	b := &envelope.Builder{Repo: *repo, Outbox: *outbox}
	env, err := b.Build(sha)
	if err != nil {
		return err
	}
	path, err := envelope.Write(*outbox, env)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", sha, path)
	return nil
}

func cmdAmend(args []string) error {
	fs, repo, outbox, _ := newSet("amend")
	msg := fs.String("m", "", "new commit message (empty keeps it)")
	sign := fs.Bool("S", true, "sign the commit")
	program := fs.String("g", "", "gpg program for signing (default: repository config)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	sha, err := gitx.Amend(*repo, *msg, *sign, *program)
	if err != nil {
		return err
	}
	b := &envelope.Builder{Repo: *repo, Outbox: *outbox}
	env, err := b.Build(sha)
	if err != nil {
		return err
	}
	path, err := envelope.Write(*outbox, env)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", sha, path)
	return nil
}

func cmdRm(args []string) error {
	fs, repo, outbox, _ := newSet("rm")
	msg := fs.String("m", "route: rm", "commit message")
	sign := fs.Bool("S", true, "sign the commit")
	program := fs.String("g", "", "gpg program for signing (default: repository config)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("rm: expected at least one path")
	}
	rmArgs := append([]string{"rm", "-r", "--"}, fs.Args()...)
	if _, err := gitx.Run(*repo, rmArgs...); err != nil {
		return err
	}
	sha, err := gitx.CommitStaged(*repo, *msg, *sign, *program)
	if err != nil {
		return err
	}
	b := &envelope.Builder{Repo: *repo, Outbox: *outbox}
	env, err := b.Build(sha)
	if err != nil {
		return err
	}
	path, err := envelope.Write(*outbox, env)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", sha, path)
	return nil
}

func cmdInstallHooks(args []string) error {
	fs := flag.NewFlagSet("install-hooks", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repo := fs.String("C", ".", "repository")
	hooks := fs.String("H", "hooks", "hooks directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	abs, err := filepath.Abs(*hooks)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Chmod(filepath.Join(abs, e.Name()), 0o755); err != nil {
			return err
		}
	}
	if err := gitx.Config(*repo, "core.hooksPath", abs); err != nil {
		return err
	}
	fmt.Printf("core.hooksPath=%s\n", abs)
	return nil
}
