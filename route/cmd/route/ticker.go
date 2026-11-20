package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"route/internal/ticker"
)

// cmdTicker serves the two market-data endpoints from a fixture file.
func cmdTicker(args []string) error {
	fs := flag.NewFlagSet("ticker", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("f", "markets.json", "fixture file with markets and assets")
	addr := fs.String("addr", "127.0.0.1:8000", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	data, err := ticker.Load(*file)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "ticker: http://%s/markets.json\n", *addr)
	return http.ListenAndServe(*addr, ticker.Handler(data))
}
