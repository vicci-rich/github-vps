package xmlio

import (
	"strings"
	"testing"
)

func validEnvelope() *Envelope {
	return &Envelope{
		Sha:         "9f2c1d7b3a4e5f60718293a4b5c6d7e8f9012345",
		Author:      "ops@example.org",
		Fpr:         "7C4A8B2F1E9D0C3B6A5F4E3D2C1B0A9988776655",
		Tree:        "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567",
		CommittedAt: "2026-11-20T10:00:00Z",
		Verified:    "true",
		Diffstat:    &Diffstat{Files: 3, Insertions: 41, Deletions: 7},
		GraphDigest: strings.Repeat("a", 64),
	}
}

func TestRoundTrip(t *testing.T) {
	env := validEnvelope()
	if err := Validate(env); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}
	body, err := Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := doc.(*Envelope)
	if !ok {
		t.Fatalf("parsed %T, want *Envelope", doc)
	}
	if got.Sha != env.Sha || got.Author != env.Author || got.Diffstat.Files != 3 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if err := Validate(got); err != nil {
		t.Fatalf("round-tripped envelope rejected: %v", err)
	}
}

func TestNamespacedDocument(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><envelope xmlns="urn:route:1" sha="9f2c1d7b3a4e5f60718293a4b5c6d7e8f9012345" author="a@b.c" committedAt="2026-11-20T10:00:00Z" verified="not-recomputed"/>`)
	doc, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(doc); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		doc  any
		want string
	}{
		{"short sha", &Envelope{Sha: "abc", Author: "a@b.c", CommittedAt: "2026-11-20T10:00:00Z", Verified: "true"}, "envelope.sha"},
		{"bad email", &Envelope{Sha: "9f2c1d7b3a", Author: "nope", CommittedAt: "2026-11-20T10:00:00Z", Verified: "true"}, "envelope.author"},
		{"bad date", &Envelope{Sha: "9f2c1d7b3a", Author: "a@b.c", CommittedAt: "yesterday", Verified: "true"}, "envelope.committedAt"},
		{"bad verified", &Envelope{Sha: "9f2c1d7b3a", Author: "a@b.c", CommittedAt: "2026-11-20T10:00:00Z", Verified: "maybe"}, "envelope.verified"},
		{"bad verb", &Operation{Verb: "frobnicate", Contract: "source.sol", Fn: "creatPig"}, "operation.verb"},
		{"missing contract", &Operation{Verb: "create", Fn: "creatPig"}, "operation.contract"},
		{"bad state", &Record{ID: 1, Entity: "pig", State: "destroy"}, "record.state"},
		{"zero id", &Record{ID: 0, Entity: "pig", State: "create"}, "record.id"},
		{"bad field type", &Operation{Verb: "create", Contract: "c", Fn: "f", Params: []Field{{Name: "x", Type: "float"}}}, "operation.param.type"},
		{"bad proof", &Proof{Sha: "9f2c1d7b3a", GPG: &GPGProof{Verified: "trust-me"}}, "proof.gpg.verified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.doc)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestParseUnknownRoot(t *testing.T) {
	if _, err := Parse([]byte(`<widget/>`)); err == nil {
		t.Fatal("expected an error for an unknown root element")
	}
}

func TestAllDocumentKinds(t *testing.T) {
	docs := []any{
		&Record{ID: 7, Entity: "pig", State: "change", Fields: []Field{{Name: "weight", Type: "uint256", Value: "120"}},
			EnvelopeRef: &EnvelopeRef{Sha: "9f2c1d7b3a"}, ChainRefs: []ChainRef{{Chain: "justitia", Block: "12", Tx: "ab"}}},
		&Operation{Verb: "update", Contract: "source.sol", Fn: "changePigInfo",
			Params: []Field{{Name: "id", Type: "uint256", Value: "7"}}, EnvelopeRef: &EnvelopeRef{Sha: "9f2c1d7b3a"}},
		&Proof{Sha: "9f2c1d7b3a", GPG: &GPGProof{Fpr: "aa", Verified: "true"},
			Receipt: &ReceiptProof{Root: "bb", Verified: "not-recomputed"}},
		&Endpoint{Name: "lightnode", URL: "ws://127.0.0.1:8080/ws", Protocol: "ws"},
	}
	for _, doc := range docs {
		if err := Validate(doc); err != nil {
			t.Fatalf("Validate(%T): %v", doc, err)
		}
		body, err := Marshal(doc)
		if err != nil {
			t.Fatalf("Marshal(%T): %v", doc, err)
		}
		back, err := Parse(body)
		if err != nil {
			t.Fatalf("Parse(%T): %v", doc, err)
		}
		if err := Validate(back); err != nil {
			t.Fatalf("Validate(round trip %T): %v", back, err)
		}
	}
}

func TestMarshalRejectsUnencodable(t *testing.T) {
	if _, err := Marshal(make(chan int)); err == nil {
		t.Fatal("expected an error marshalling a channel")
	}
}

func TestValidateUnsupported(t *testing.T) {
	if err := Validate(struct{}{}); err == nil {
		t.Fatal("expected an error for an unsupported document")
	}
}
