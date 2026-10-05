package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRoundTrip(t *testing.T) {
	in := Config{
		Provider:         "aws",
		ProviderProfile:  "aws_profile",
		ProjectName:      "name",
		VariableBasePath: "/configurator/name",
	}
	out, err := Parse(strings.NewReader(string(in.Encode())))
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip = %+v, want %+v", out, in)
	}
}

func TestParseCommentsAndQuotes(t *testing.T) {
	c, err := Parse(strings.NewReader("# comment\n\nPROVIDER = \"aws\"\nPROJECT_NAME='p'\nVARIABLE_BASE_PATH=/x/\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider != "aws" || c.ProjectName != "p" || c.BasePath() != "/x" {
		t.Fatalf("got %+v", c)
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"NOPE", "UNKNOWN=1"} {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

func TestValidate(t *testing.T) {
	valid := Config{Provider: "aws", ProjectName: "p", VariableBasePath: "/configurator/p"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for _, bp := range []string{"", "relative", "/has space", `/quote"`} {
		c := valid
		c.VariableBasePath = bp
		if err := c.Validate(); err == nil {
			t.Errorf("base path %q accepted", bp)
		}
	}
}

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(nested); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Find before create: %v", err)
	}
	want := filepath.Join(root, FileName)
	if err := os.WriteFile(want, []byte("PROVIDER=aws\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}
	// Resolve symlinks (macOS /var -> /private/var) before comparing.
	got, _ = filepath.EvalSymlinks(got)
	want, _ = filepath.EvalSymlinks(want)
	if got != want {
		t.Fatalf("Find = %q, want %q", got, want)
	}
}
