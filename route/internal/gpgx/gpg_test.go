package gpgx

import (
	"strings"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	home, err := NewHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()

	fpr, err := home.GenerateKey("Route Test", "route@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(fpr) != 40 {
		t.Fatalf("fingerprint length = %d (%q)", len(fpr), fpr)
	}

	payload := []byte("route: canonical envelope payload\n")
	sig, err := home.Sign(payload, fpr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sig, "BEGIN PGP SIGNATURE") {
		t.Fatalf("signature is not armored: %q", sig)
	}

	got, ok, err := home.Verify(payload, sig)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != fpr {
		t.Fatalf("verify = (%q, %t), want (%q, true)", got, ok, fpr)
	}

	if _, ok, _ := home.Verify([]byte("tampered"), sig); ok {
		t.Fatal("tampered payload verified")
	}
}

func TestExportImport(t *testing.T) {
	signer, err := NewHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer signer.Close()
	fpr, err := signer.GenerateKey("Pinned", "pinned@example.com")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := signer.Export(fpr)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("pinned keyring payload")
	sig, err := signer.Sign(payload, fpr)
	if err != nil {
		t.Fatal(err)
	}

	verifier, err := NewHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer verifier.Close()
	if err := verifier.Import(pub); err != nil {
		t.Fatal(err)
	}
	got, ok, err := verifier.Verify(payload, sig)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != fpr {
		t.Fatalf("pinned verify = (%q, %t), want (%q, true)", got, ok, fpr)
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	home, err := NewHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	if _, ok, _ := home.Verify([]byte("x"), "not a signature"); ok {
		t.Fatal("garbage signature verified")
	}
	if err := home.Import("not a key"); err == nil {
		t.Fatal("expected an error importing garbage")
	}
}
