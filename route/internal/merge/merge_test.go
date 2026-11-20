package merge

import (
	"errors"
	"strings"
	"testing"
)

// fakeRecover treats the signature as the address itself, so tests can state
// exactly who signed.
func fakeRecover(_ []byte, sig []byte) (string, error) {
	if len(sig) == 0 {
		return "", errors.New("empty signature")
	}
	return string(sig), nil
}

func policy2of2() Policy {
	return Policy{Threshold: 2, Keys: []Signer{{Address: "alice", Weight: 1}, {Address: "bob", Weight: 1}}}
}

func TestTwoOfTwo(t *testing.T) {
	digest := Digest([]byte("envelope"))
	sum, ok, err := policy2of2().Approve(digest, []Vote{{Signature: []byte("alice")}, {Signature: []byte("bob")}}, fakeRecover)
	if err != nil || !ok || sum != 2 {
		t.Fatalf("Approve = (%d,%t,%v), want (2,true,nil)", sum, ok, err)
	}
}

func TestOneVoteIsNotEnough(t *testing.T) {
	sum, ok, err := policy2of2().Approve(Digest(nil), []Vote{{Signature: []byte("alice")}}, fakeRecover)
	if err != nil || ok || sum != 1 {
		t.Fatalf("Approve = (%d,%t,%v), want (1,false,nil)", sum, ok, err)
	}
}

func TestDuplicatesAndStrangersAreRejected(t *testing.T) {
	_, _, err := policy2of2().Approve(Digest(nil), []Vote{
		{Signature: []byte("alice")}, {Signature: []byte("alice")},
	}, fakeRecover)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate vote: %v", err)
	}

	_, _, err = policy2of2().Approve(Digest(nil), []Vote{{Signature: []byte("mallory")}}, fakeRecover)
	if err == nil || !strings.Contains(err.Error(), "not in the policy") {
		t.Fatalf("stranger vote: %v", err)
	}
}

func TestClaimedSignerMustMatchTheSignature(t *testing.T) {
	_, _, err := policy2of2().Approve(Digest(nil), []Vote{
		{Signer: "bob", Signature: []byte("alice")},
	}, fakeRecover)
	if err == nil || !strings.Contains(err.Error(), "claims") {
		t.Fatalf("mismatched signer: %v", err)
	}
}

func TestBadPolicy(t *testing.T) {
	cases := map[string]Policy{
		"zero threshold": {Threshold: 0, Keys: []Signer{{Address: "a", Weight: 1}}},
		"no keys":        {Threshold: 1},
		"zero weight":    {Threshold: 1, Keys: []Signer{{Address: "a", Weight: 0}}},
		"duplicate key":  {Threshold: 1, Keys: []Signer{{Address: "a", Weight: 1}, {Address: "a", Weight: 1}}},
		"threshold>sum":  {Threshold: 3, Keys: []Signer{{Address: "a", Weight: 1}}},
	}
	for name, p := range cases {
		if err := p.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	if _, _, err := policy2of2().Approve(nil, nil, nil); err == nil {
		t.Fatal("nil recover must be rejected")
	}
}

func TestDigestIsStable(t *testing.T) {
	a := Digest([]byte("x"))
	b := Digest([]byte("x"))
	if len(a) != 32 || string(a) != string(b) {
		t.Fatal("digest is not a stable 32 bytes")
	}
	if string(Digest([]byte("y"))) == string(a) {
		t.Fatal("different payloads collided")
	}
}
