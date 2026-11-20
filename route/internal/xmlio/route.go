// Package xmlio implements the public-domain interface defined in
// interface/route.xsd: parsing, marshalling and structural validation of the
// five documents that travel between the git backend, the CRUD daemon and the
// light-node web app.
package xmlio

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"time"
)

// Diffstat mirrors <diffstat files="" insertions="" deletions=""/>.
type Diffstat struct {
	Files      int `xml:"files,attr"`
	Insertions int `xml:"insertions,attr"`
	Deletions  int `xml:"deletions,attr"`
}

// Envelope is a signed commit: the record of truth produced by the backend.
type Envelope struct {
	XMLName     xml.Name  `xml:"envelope"`
	Sha         string    `xml:"sha,attr"`
	Author      string    `xml:"author,attr"`
	Fpr         string    `xml:"fpr,attr,omitempty"`
	Tree        string    `xml:"tree,attr,omitempty"`
	Parents     string    `xml:"parents,attr,omitempty"`
	CommittedAt string    `xml:"committedAt,attr"`
	Verified    string    `xml:"verified,attr"`
	Sig         string    `xml:"sig,omitempty"`
	Diffstat    *Diffstat `xml:"diffstat,omitempty"`
	GraphDigest string    `xml:"graphDigest,omitempty"`
}

// Field is a typed value carried by a record or an operation parameter.
type Field struct {
	XMLName xml.Name `xml:"field"`
	Name    string   `xml:"name,attr"`
	Type    string   `xml:"type,attr"`
	Value   string   `xml:",chardata"`
}

// ChainRef points at where a record was observed on a chain.
type ChainRef struct {
	Chain       string `xml:"chain,attr"`
	Block       string `xml:"block,attr"`
	Tx          string `xml:"tx,attr"`
	ReceiptRoot string `xml:"receiptRoot,attr,omitempty"`
	TxIndex     *int   `xml:"txIndex,attr,omitempty"`
}

// EnvelopeRef names the commit that produced a record or operation.
type EnvelopeRef struct {
	Sha string `xml:"sha,attr"`
}

// Record is one CRUD row as served to the client.
type Record struct {
	XMLName     xml.Name     `xml:"record"`
	ID          int          `xml:"id,attr"`
	Entity      string       `xml:"entity,attr"`
	State       string       `xml:"state,attr"`
	Chain       string       `xml:"chain,attr,omitempty"`
	Fields      []Field      `xml:"field"`
	EnvelopeRef *EnvelopeRef `xml:"envelopeRef,omitempty"`
	ChainRefs   []ChainRef   `xml:"chainRef"`
}

// Operation is a write request from the client towards the chain.
type Operation struct {
	XMLName     xml.Name     `xml:"operation"`
	Verb        string       `xml:"verb,attr"`
	Contract    string       `xml:"contract,attr"`
	Fn          string       `xml:"fn,attr"`
	Address     string       `xml:"address,attr,omitempty"`
	Params      []Field      `xml:"param"`
	EnvelopeRef *EnvelopeRef `xml:"envelopeRef,omitempty"`
}

// GPGProof records the result of verifying a commit signature.
type GPGProof struct {
	Fpr      string `xml:"fpr,attr,omitempty"`
	Verified string `xml:"verified,attr"`
}

// ReceiptProof records the result of checking a receipt against the header.
type ReceiptProof struct {
	Root          string `xml:"root,attr,omitempty"`
	TxIndex       *int   `xml:"txIndex,attr,omitempty"`
	HeaderSigData string `xml:"headerSigData,attr,omitempty"`
	Verified      string `xml:"verified,attr"`
}

// Proof is the evidence document served by the light node.
type Proof struct {
	XMLName xml.Name      `xml:"proof"`
	Sha     string        `xml:"sha,attr"`
	GPG     *GPGProof     `xml:"gpg,omitempty"`
	Receipt *ReceiptProof `xml:"receipt,omitempty"`
}

// Endpoint describes one service the client may call.
type Endpoint struct {
	XMLName  xml.Name `xml:"endpoint"`
	Name     string   `xml:"name,attr"`
	URL      string   `xml:"url,attr"`
	Protocol string   `xml:"protocol,attr"`
}

var (
	shaRE      = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	emailRE    = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)
	verbs      = map[string]bool{"create": true, "read": true, "update": true, "delete": true, "transfer": true, "watch": true, "submit": true, "approve": true, "deposit": true}
	states     = map[string]bool{"create": true, "change": true, "transfer": true, "remove": true}
	fieldTypes = map[string]bool{"address": true, "uint256": true, "uint64": true, "string": true, "bool": true, "bytes": true}
	verified   = map[string]bool{"true": true, "false": true, "not-recomputed": true}
)

// Parse decodes any of the five documents and returns one of *Envelope,
// *Record, *Operation, *Proof, *Endpoint.
func Parse(data []byte) (any, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var root xml.Name
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("xml: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			root = se.Name
			break
		}
	}
	switch root.Local {
	case "envelope":
		var v Envelope
		if err := xml.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case "record":
		var v Record
		if err := xml.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case "operation":
		var v Operation
		if err := xml.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case "proof":
		var v Proof
		if err := xml.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case "endpoint":
		var v Endpoint
		if err := xml.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}
	return nil, fmt.Errorf("xml: unknown root element %q", root.Local)
}

// Marshal encodes a document with the XML declaration and indentation.
func Marshal(v any) ([]byte, error) {
	body, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.Write(body)
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// Validate applies the rules of interface/route.xsd and returns the first
// violation as an error that names the offending field.
func Validate(v any) error {
	switch d := v.(type) {
	case *Envelope:
		if !shaRE.MatchString(d.Sha) {
			return fmt.Errorf("envelope.sha: %q is not a commit sha", d.Sha)
		}
		if !emailRE.MatchString(d.Author) {
			return fmt.Errorf("envelope.author: %q is not an email", d.Author)
		}
		if _, err := time.Parse(time.RFC3339, d.CommittedAt); err != nil {
			return fmt.Errorf("envelope.committedAt: %w", err)
		}
		if !verified[d.Verified] {
			return fmt.Errorf("envelope.verified: %q not in {true,false,not-recomputed}", d.Verified)
		}
	case *Record:
		if d.ID <= 0 {
			return fmt.Errorf("record.id: must be positive, got %d", d.ID)
		}
		if d.Entity == "" {
			return fmt.Errorf("record.entity: required")
		}
		if !states[d.State] {
			return fmt.Errorf("record.state: %q not in {create,change,transfer,remove}", d.State)
		}
		for i := range d.Fields {
			if err := validateField("record.field", &Field{Name: d.Fields[i].Name, Type: d.Fields[i].Type}); err != nil {
				return err
			}
		}
	case *Operation:
		if !verbs[d.Verb] {
			return fmt.Errorf("operation.verb: %q not allowed", d.Verb)
		}
		if d.Contract == "" {
			return fmt.Errorf("operation.contract: required")
		}
		if d.Fn == "" {
			return fmt.Errorf("operation.fn: required")
		}
		for i := range d.Params {
			if err := validateField("operation.param", &d.Params[i]); err != nil {
				return err
			}
		}
	case *Proof:
		if !shaRE.MatchString(d.Sha) {
			return fmt.Errorf("proof.sha: %q is not a commit sha", d.Sha)
		}
		if d.GPG != nil && !verified[d.GPG.Verified] {
			return fmt.Errorf("proof.gpg.verified: %q not in {true,false,not-recomputed}", d.GPG.Verified)
		}
		if d.Receipt != nil && !verified[d.Receipt.Verified] {
			return fmt.Errorf("proof.receipt.verified: %q not in {true,false,not-recomputed}", d.Receipt.Verified)
		}
	case *Endpoint:
		if d.Name == "" || d.URL == "" || d.Protocol == "" {
			return fmt.Errorf("endpoint: name, url and protocol are required")
		}
	default:
		return fmt.Errorf("validate: unsupported document %T", v)
	}
	return nil
}

func validateField(path string, f *Field) error {
	if f.Name == "" {
		return fmt.Errorf("%s.name: required", path)
	}
	if !fieldTypes[f.Type] {
		return fmt.Errorf("%s.type: %q not in {address,uint256,uint64,string,bool,bytes}", path, f.Type)
	}
	return nil
}
