package main

import (
	"flag"
	"fmt"
	"os"

	"route/internal/audit"
)

// cmdAudit checks the duplication policy declared in inventory.json.
func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("f", "inventory.json", "inventory file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	inv, err := audit.Load(*file)
	if err != nil {
		return err
	}
	problems := audit.Check(inv)
	if len(problems) == 0 {
		fmt.Println("audit: OK")
		return nil
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	return fmt.Errorf("audit: %d violation(s)", len(problems))
}
