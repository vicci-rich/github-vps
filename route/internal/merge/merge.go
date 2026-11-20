// Package merge implements the m-of-n signature accounting the merge step
// needs: a weight per signer, a threshold, duplicate and unknown signer
// rejection — the same rules as Tron's TransactionCapsule.checkWeight, but
// over an arbitrary recover function so the caller decides the curve.
package merge

import (
	"crypto/sha256"
	"fmt"
)

// Signer is one key in the policy.
type Signer struct {
	Address string
	Weight  int
}

// Policy is the m-of-n key set and its threshold.
type Policy struct {
	Threshold int
	Keys      []Signer
}

// Vote is one signature over the merge digest.
type Vote struct {
	Signer    string
	Signature []byte
}

// Recover maps a digest and a signature to a signer address.
type Recover func(digest []byte, signature []byte) (string, error)

// Digest binds a payload (the canonical envelope) to a 32-byte digest.
func Digest(payload []byte) []byte {
	sum := sha256.Sum256(payload)
	return sum[:]
}

// Validate checks the policy itself: positive threshold, at least one key,
// positive weights, unique addresses, and threshold <= total weight.
func (p Policy) Validate() error {
	if p.Threshold <= 0 {
		return fmt.Errorf("merge: threshold must be positive")
	}
	if len(p.Keys) == 0 {
		return fmt.Errorf("merge: no keys")
	}
	seen := map[string]bool{}
	total := 0
	for _, k := range p.Keys {
		if k.Address == "" {
			return fmt.Errorf("merge: key without address")
		}
		if k.Weight <= 0 {
			return fmt.Errorf("merge: key %s has non-positive weight", k.Address)
		}
		if seen[k.Address] {
			return fmt.Errorf("merge: duplicate key %s", k.Address)
		}
		seen[k.Address] = true
		total += k.Weight
	}
	if p.Threshold > total {
		return fmt.Errorf("merge: threshold %d exceeds total weight %d", p.Threshold, total)
	}
	return nil
}

// Approve counts the distinct signer weights over the digest and reports
// whether the threshold is met. A bad signature is an error, not a silent
// "no": the caller must see a forged vote.
func (p Policy) Approve(digest []byte, votes []Vote, recover Recover) (int, bool, error) {
	if err := p.Validate(); err != nil {
		return 0, false, err
	}
	if recover == nil {
		return 0, false, fmt.Errorf("merge: nil recover function")
	}
	weight := map[string]int{}
	for _, k := range p.Keys {
		weight[k.Address] = k.Weight
	}
	seen := map[string]bool{}
	sum := 0
	for _, v := range votes {
		addr, err := recover(digest, v.Signature)
		if err != nil {
			return 0, false, fmt.Errorf("merge: recover vote: %w", err)
		}
		if v.Signer != "" && v.Signer != addr {
			return 0, false, fmt.Errorf("merge: vote claims %s but signature recovers %s", v.Signer, addr)
		}
		if seen[addr] {
			return 0, false, fmt.Errorf("merge: duplicate signer %s", addr)
		}
		w, ok := weight[addr]
		if !ok {
			return 0, false, fmt.Errorf("merge: signer %s is not in the policy", addr)
		}
		seen[addr] = true
		sum += w
	}
	return sum, sum >= p.Threshold, nil
}
