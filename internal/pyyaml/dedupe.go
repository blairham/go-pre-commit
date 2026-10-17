// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package pyyaml

import (
	"gopkg.in/yaml.v3"
)

// Unmarshal is yaml.Unmarshal reading a mapping as PyYAML's SafeLoader
// does: a key repeated in one mapping is one entry, at its first position,
// holding its last value -- a Python dict -- rather than yaml.v3's
// "mapping key already defined" error.
func Unmarshal(data []byte, out any) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind == 0 {
		return nil
	}
	dedupeKeys(&doc, map[*yaml.Node]bool{})
	return doc.Decode(out)
}

// dedupeKeys rewrites every mapping under n so each key appears once.
func dedupeKeys(n *yaml.Node, seen map[*yaml.Node]bool) {
	if n == nil || seen[n] {
		return
	}
	seen[n] = true
	for _, c := range n.Content {
		dedupeKeys(c, seen)
	}
	dedupeKeys(n.Alias, seen)
	if n.Kind != yaml.MappingNode {
		return
	}
	c := &constructor{built: map[*yaml.Node]*Value{}}
	var kept []*yaml.Node
	var keys []*Value
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		var kv *Value
		if deref(k).Kind == yaml.ScalarNode && !isMergeKey(deref(k)) {
			// A scalar always constructs; on any error the key is
			// simply not compared.
			if built, err := c.construct(k); err == nil {
				kv = built
			}
		}
		replaced := false
		if kv != nil {
			for j, prev := range keys {
				if prev != nil && sameKey(prev, kv) {
					kept[2*j+1] = v
					replaced = true
					break
				}
			}
		}
		if !replaced {
			kept = append(kept, k, v)
			keys = append(keys, kv)
		}
	}
	n.Content = kept
}

func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}
