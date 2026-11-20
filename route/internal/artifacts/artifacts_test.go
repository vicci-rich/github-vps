package artifacts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeS3 implements the small surface minio-go needs (HEAD/PUT bucket,
// PUT/GET object). Signatures are ignored: the point is our key/prefix logic
// and the HTTP contract, not AWS auth.
type fakeS3 struct {
	mu      sync.Mutex
	buckets map[string]bool
	objects map[string][]byte
}

func newFakeS3() *fakeS3 {
	return &fakeS3{buckets: map[string]bool{}, objects: map[string][]byte{}}
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	switch {
	case parts[0] == "":
		http.Error(w, "no bucket", http.StatusBadRequest)
	case len(parts) == 1 && r.Method == http.MethodHead:
		if f.buckets[bucket] {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	case len(parts) == 1 && r.Method == http.MethodPut:
		f.buckets[bucket] = true
		w.WriteHeader(http.StatusOK)
	case len(parts) == 2 && r.Method == http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.objects[bucket+"/"+parts[1]] = body
		w.Header().Set("ETag", "\"fake\"")
		w.WriteHeader(http.StatusOK)
	case len(parts) == 2 && r.Method == http.MethodGet:
		body, ok := f.objects[bucket+"/"+parts[1]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", "\"fake\"")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	default:
		http.Error(w, "unsupported", http.StatusNotImplemented)
	}
}

func TestKeyJoin(t *testing.T) {
	cases := map[string]string{
		"":        "abi/x.json",
		"demo":    "demo/abi/x.json",
		"demo/":   "demo/abi/x.json",
		"/demo//": "demo/abi/x.json",
		"a/b":     "a/b/abi/x.json",
	}
	for prefix, want := range cases {
		if got := (&Store{Prefix: prefix}).Key("abi/x.json"); got != want {
			t.Errorf("prefix %q: key = %q, want %q", prefix, got, want)
		}
	}
	if got := (&Store{}).Key("/leading.json"); got != "leading.json" {
		t.Errorf("leading slash not trimmed: %q", got)
	}
}

func TestRoundTripAgainstS3Surface(t *testing.T) {
	fake := newFakeS3()
	srv := httptest.NewServer(fake)
	defer srv.Close()

	s, err := Open(strings.TrimPrefix(srv.URL, "http://"), "route", "routeroute",
		"route-bucket", "demo", false)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"abi":[{"name":"changePigInfo"}]}`)
	key, err := s.Put(context.Background(), "abi/source.json", body)
	if err != nil {
		t.Fatal(err)
	}
	if key != "demo/abi/source.json" {
		t.Fatalf("key = %q", key)
	}
	got, err := s.Get(context.Background(), "abi/source.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("round trip mismatch: %q", got)
	}
	if _, err := s.Get(context.Background(), "missing.json"); err == nil {
		t.Fatal("expected an error for a missing object")
	}
}

func TestOpenValidation(t *testing.T) {
	if _, err := Open("", "a", "b", "c", "", false); err == nil {
		t.Fatal("expected an error without an endpoint")
	}
	if _, err := Open("127.0.0.1:1", "a", "b", "", "", false); err == nil {
		t.Fatal("expected an error without a bucket")
	}
	if contentType("x.json") != "application/json" {
		t.Fatal("json content type")
	}
	if contentType("x.bin") == "application/json" {
		t.Fatal("binary content type")
	}
}
