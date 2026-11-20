// Package audit checks the duplication policy: every declared job must have
// exactly the number of implementations the inventory allows. It turns the
// "one watcher, one store, two callers" rule into a testable assertion.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Constraint is the expected number of implementations for a job.
type Constraint struct {
	Job     string `json:"job"`
	Exactly int    `json:"exactly"`
}

// Inventory lists the implementations per job and the constraints.
type Inventory struct {
	Jobs        map[string][]string `json:"jobs"`
	Constraints []Constraint        `json:"constraints"`
}

// Load reads an inventory file.
func Load(path string) (*Inventory, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var inv Inventory
	if err := json.Unmarshal(body, &inv); err != nil {
		return nil, fmt.Errorf("audit: %s: %w", path, err)
	}
	return &inv, nil
}

// Check returns one message per violation, sorted by job name.
func Check(inv *Inventory) []string {
	if inv == nil {
		return []string{"audit: nil inventory"}
	}
	var problems []string
	jobs := make([]string, 0, len(inv.Constraints))
	for _, c := range inv.Constraints {
		jobs = append(jobs, c.Job)
	}
	sort.Strings(jobs)
	for _, job := range jobs {
		for _, c := range inv.Constraints {
			if c.Job != job {
				continue
			}
			got := len(inv.Jobs[c.Job])
			if got != c.Exactly {
				problems = append(problems, fmt.Sprintf(
					"job %q has %d implementation(s), want exactly %d: %v",
					c.Job, got, c.Exactly, inv.Jobs[c.Job]))
			}
		}
	}
	for _, c := range inv.Constraints {
		if c.Exactly < 0 {
			problems = append(problems, fmt.Sprintf("job %q: negative constraint", c.Job))
		}
	}
	return problems
}
