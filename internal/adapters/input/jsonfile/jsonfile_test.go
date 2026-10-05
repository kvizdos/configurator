package jsonfile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kvizdos/configurator/internal/core"
)

func TestParse(t *testing.T) {
	in := `{
		"db_host": "db.internal",
		"db_port": 5432,
		"debug": false,
		"db_pass": {"value": "hunter2", "sensitive": true},
		"region": {"value": "us-east-1"}
	}`
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Variable{
		{Key: "db_host", Value: "db.internal"},
		{Key: "db_pass", Value: "hunter2", Sensitive: true},
		{Key: "db_port", Value: "5432"},
		{Key: "debug", Value: "false"},
		{Key: "region", Value: "us-east-1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	for name, in := range map[string]string{
		"not an object":  `["a"]`,
		"null value":     `{"a": null}`,
		"array value":    `{"a": [1]}`,
		"missing value":  `{"a": {"sensitive": true}}`,
		"nested object":  `{"a": {"value": {"x": 1}}}`,
		"unknown field":  `{"a": {"value": "x", "secret": true}}`,
		"malformed json": `{"a": `,
	} {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("%s: Parse(%s) succeeded, want error", name, in)
		}
	}
}
