package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"route/internal/proof"
	"route/internal/xmlio"
)

func cmdProof(args []string) error {
	fs := flag.NewFlagSet("proof", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repo := fs.String("C", ".", "repository (empty disables GPG recomputation)")
	outbox := fs.String("o", "outbox", "outbox directory")
	keys := fs.String("k", "keys", "pinned keyring directory")
	receipts := fs.String("receipts", "", "optional receipts JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("proof: expected one sha")
	}
	rm, err := proof.LoadReceipts(*receipts)
	if err != nil {
		return err
	}
	s := &proof.Store{Repo: *repo, Outbox: *outbox, Keys: *keys, Receipts: rm}
	doc, err := s.Build(fs.Arg(0))
	if err != nil {
		return err
	}
	body, err := xmlio.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(body)
	return err
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repo := fs.String("C", ".", "repository (empty disables GPG recomputation)")
	outbox := fs.String("o", "outbox", "outbox directory")
	keys := fs.String("k", "keys", "pinned keyring directory")
	receipts := fs.String("receipts", "", "optional receipts JSON")
	addr := fs.String("addr", "127.0.0.1:8081", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rm, err := proof.LoadReceipts(*receipts)
	if err != nil {
		return err
	}
	s := &proof.Store{Repo: *repo, Outbox: *outbox, Keys: *keys, Receipts: rm}
	fmt.Fprintf(os.Stderr, "proof: http://%s/proof/{sha}\n", *addr)
	return http.ListenAndServe(*addr, proof.Handler(s))
}
