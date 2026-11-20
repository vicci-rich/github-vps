package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"route/internal/artifacts"
)

func cmdArtifacts(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("artifacts: expected put or get")
	}
	switch args[0] {
	case "put":
		return artifactsPut(args[1:])
	case "get":
		return artifactsGet(args[1:])
	}
	return fmt.Errorf("artifacts: unknown verb %q", args[0])
}

func artifactsFlags(name string) (*flag.FlagSet, *string, *string, *string, *bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	endpoint := fs.String("endpoint", "127.0.0.1:9000", "S3-compatible endpoint (host:port)")
	bucket := fs.String("bucket", "route", "bucket (no '/')")
	prefix := fs.String("prefix", "demo", "key prefix")
	secure := fs.Bool("secure", false, "use https")
	return fs, endpoint, bucket, prefix, secure
}

// openStore reads credentials from the environment only.
func openStore(endpoint, bucket, prefix string, secure bool) (*artifacts.Store, error) {
	access := os.Getenv("ROUTE_S3_ACCESS_KEY")
	secret := os.Getenv("ROUTE_S3_SECRET_KEY")
	if access == "" || secret == "" {
		return nil, fmt.Errorf("set ROUTE_S3_ACCESS_KEY and ROUTE_S3_SECRET_KEY")
	}
	return artifacts.Open(endpoint, access, secret, bucket, prefix, secure)
}

func artifactsPut(args []string) error {
	fs, endpoint, bucket, prefix, secure := artifactsFlags("artifacts put")
	file := fs.String("f", "", "file to upload")
	name := fs.String("name", "", "object name, e.g. abi/source.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" || *name == "" {
		return fmt.Errorf("artifacts put: -f and -name are required")
	}
	body, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	store, err := openStore(*endpoint, *bucket, *prefix, *secure)
	if err != nil {
		return err
	}
	key, err := store.Put(context.Background(), *name, body)
	if err != nil {
		return err
	}
	fmt.Printf("%s/%s\n", *bucket, key)
	return nil
}

func artifactsGet(args []string) error {
	fs, endpoint, bucket, prefix, secure := artifactsFlags("artifacts get")
	name := fs.String("name", "", "object name")
	out := fs.String("out", "", "write to this file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("artifacts get: -name is required")
	}
	store, err := openStore(*endpoint, *bucket, *prefix, *secure)
	if err != nil {
		return err
	}
	body, err := store.Get(context.Background(), *name)
	if err != nil {
		return err
	}
	if *out == "" {
		_, err = os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(*out, body, 0o644)
}
