package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"route/internal/chaininfo"
)

// cmdChaininfo reads the gateway with the second application caller.
func cmdChaininfo(args []string) error {
	fs := flag.NewFlagSet("chaininfo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	rpc := fs.String("rpc", "http://127.0.0.1:47768", "JSON-RPC gateway")
	address := fs.String("address", "", "account to inspect (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := chaininfo.New(*rpc).Read(*address)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}
