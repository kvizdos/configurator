// Package jsonfile parses the JSON documents accepted by `configurator load`.
//
// The document is an object whose values are either a scalar (stored as a
// non-sensitive string) or an object with "value" and optional "sensitive":
//
//	{
//	  "db_host": "db.internal",
//	  "db_port": 5432,
//	  "db_pass": {"value": "hunter2", "sensitive": true}
//	}
package jsonfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/kvizdos/configurator/internal/core"
)

type entry struct {
	Value     *json.RawMessage `json:"value"`
	Sensitive bool             `json:"sensitive"`
}

// Parse returns the variables in r, sorted by key.
func Parse(r io.Reader) ([]core.Variable, error) {
	var doc map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	vars := make([]core.Variable, 0, len(doc))
	for key, raw := range doc {
		v, err := parseEntry(key, raw)
		if err != nil {
			return nil, err
		}
		vars = append(vars, v)
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Key < vars[j].Key })
	return vars, nil
}

func parseEntry(key string, raw json.RawMessage) (core.Variable, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '{' {
		var e entry
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			return core.Variable{}, fmt.Errorf("%q: %w", key, err)
		}
		if e.Value == nil {
			return core.Variable{}, fmt.Errorf("%q: missing \"value\"", key)
		}
		value, err := scalar(key, *e.Value)
		if err != nil {
			return core.Variable{}, err
		}
		return core.Variable{Key: key, Value: value, Sensitive: e.Sensitive}, nil
	}

	value, err := scalar(key, raw)
	if err != nil {
		return core.Variable{}, err
	}
	return core.Variable{Key: key, Value: value}, nil
}

// scalar converts a JSON string, number or boolean to its string form.
func scalar(key string, raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "", fmt.Errorf("%q: empty value", key)
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", fmt.Errorf("%q: %w", key, err)
		}
		return s, nil
	case '{', '[', 'n':
		return "", fmt.Errorf("%q: value must be a string, number or boolean", key)
	default:
		return string(raw), nil
	}
}
