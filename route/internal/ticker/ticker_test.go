package ticker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "markets.json")
	body := `{"markets":[{"trade_count":3,"base_asset":{"code":"XLM"}}],"assets":[{"code":"XLM","issuer":""}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAndServe(t *testing.T) {
	d, err := Load(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(d))
	defer srv.Close()

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/markets.json", 1},
		{"/assets.json", 1},
	} {
		resp, err := http.Get(srv.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", tc.path, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("%s: content type %q", tc.path, got)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("%s: missing CORS header", tc.path)
		}
		var payload []json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if len(payload) != tc.want {
			t.Fatalf("%s: %d items, want %d", tc.path, len(payload), tc.want)
		}
	}
}

func TestUnknownPathAndMethods(t *testing.T) {
	d := &Data{}
	srv := httptest.NewServer(Handler(d))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path status = %d", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/markets.json", nil)
	optResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer optResp.Body.Close()
	if optResp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d", optResp.StatusCode)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("expected an error for invalid json")
	}
}
