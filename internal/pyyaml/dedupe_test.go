// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package pyyaml

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// A repeated key is one dict entry: first position, last value. Keys equal
// as Python values are the same key (1, 1.0 and true); merge keys are not
// deduplicated, since the loader consumes each of them.
func TestUnmarshalRepeatedKeys(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		want     []string // the top-level keys, in order, then values
	}{
		{"last value, first position", "a: 1\nb: 2\na: 3\n", []string{"a=3", "b=2"}},
		{"nested", "x:\n  k: 1\n  k: 2\n", []string{"x=map[k:2]"}},
		{"python equal keys", "1: a\n1.0: b\ntrue: c\nx: d\n", []string{"1=c", "x=d"}},
		{"quoted string is not the int", "1: a\n'1': b\n", []string{"1=a", "1=b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var n yaml.Node
			if err := Unmarshal([]byte(tc.in), &n); err != nil {
				t.Fatal(err)
			}
			m := n.Content[0]
			var got []string
			for i := 0; i+1 < len(m.Content); i += 2 {
				var v any
				if err := m.Content[i+1].Decode(&v); err != nil {
					t.Fatal(err)
				}
				got = append(got, m.Content[i].Value+"="+fmtAny(v))
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func fmtAny(v any) string {
	if m, ok := v.(map[string]any); ok {
		s := "map["
		for k, x := range m {
			s += k + ":" + fmtAny(x)
		}
		return s + "]"
	}
	b, _ := yaml.Marshal(v)
	return string(b[:len(b)-1])
}

// yaml.v3 refuses a repeated key when decoding into a struct or map;
// Unmarshal does not.
func TestUnmarshalIntoStruct(t *testing.T) {
	var out struct {
		Name string `yaml:"name"`
		Rev  string `yaml:"rev"`
	}
	in := []byte("name: first\nrev: v1\nname: second\n")
	if err := yaml.Unmarshal(in, &out); err == nil {
		t.Fatal("yaml.v3 accepted a repeated key; this test no longer proves anything")
	}
	if err := Unmarshal(in, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "second" || out.Rev != "v1" {
		t.Errorf("got %+v, want name=second rev=v1", out)
	}
}

// Merge keys keep working, and repeated merge keys are left to the decoder.
func TestUnmarshalMergeKeys(t *testing.T) {
	var out map[string]any
	in := []byte("base: &b {x: 1}\nm:\n  <<: *b\n  y: 2\n")
	if err := Unmarshal(in, &out); err != nil {
		t.Fatal(err)
	}
	m, _ := out["m"].(map[string]any)
	if m["x"] != 1 || m["y"] != 2 {
		t.Errorf("m = %v", out["m"])
	}
}

func TestUnmarshalEmpty(t *testing.T) {
	var out map[string]any
	if err := Unmarshal(nil, &out); err != nil || out != nil {
		t.Errorf("Unmarshal(empty) = %v, %v", out, err)
	}
}
