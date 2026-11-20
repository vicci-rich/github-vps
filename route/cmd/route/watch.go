package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"route/internal/store"
	"route/internal/watch"
)

// cmdWatch applies a recorded feed to the in-memory store. It is the seam the
// bds listener plugs into: same Event stream, same rows.
func cmdWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("f", "-", "feed file (JSONL); - reads stdin")
	chain := fs.String("chain", "justitia", "default chain for events")
	revert := fs.Int64("revert", 0, "revert rows at height >= N before applying")
	dsn := fs.String("dsn", "", "Postgres DSN (default: in-memory store)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var in io.Reader = os.Stdin
	if *file != "" && *file != "-" {
		f, err := os.Open(*file)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	var st store.Store
	if *dsn != "" {
		pg, err := store.Open(*dsn)
		if err != nil {
			return err
		}
		defer pg.Close()
		st = pg
	} else {
		st = store.NewMem()
	}
	if *revert > 0 {
		if err := st.RevertFrom(*chain, *revert); err != nil {
			return err
		}
	}
	stats, err := watch.Apply(st, *chain, in)
	if err != nil {
		return err
	}
	body, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

// cmdSchema prints the Postgres DDL the store expects.
func cmdSchema(args []string) error {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Print(store.Schema())
	return nil
}
