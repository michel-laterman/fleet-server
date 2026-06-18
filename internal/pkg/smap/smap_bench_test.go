// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package smap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

var benchSinkString string

// benchMap is a realistic policy-like map used across hash benchmarks.
var benchMap = Map{
	"inputs": []any{
		map[string]any{
			"type":    "logfile",
			"enabled": true,
			"streams": []any{
				map[string]any{"paths": []any{"/var/log/nginx/*.log"}},
			},
		},
	},
	"outputs": map[string]any{
		"default": map[string]any{
			"type":  "elasticsearch",
			"hosts": []any{"https://es:9200"},
		},
	},
}

// BenchmarkHash measures the current Hash() implementation:
// h.Sum(nil) allocates a 32-byte slice to hold the digest.
func BenchmarkHash(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s, err := benchMap.Hash()
		if err != nil {
			b.Fatal(err)
		}
		benchSinkString = s
	}
}

// hashStackArray is the proposed Hash() implementation.
// A stack-allocated [sha256.Size]byte array receives the digest, eliminating the heap slice.
func hashStackArray(m Map) (string, error) {
	if m == nil {
		return "", nil
	}
	h := sha256.New()
	if err := json.NewEncoder(h).Encode(m); err != nil {
		return "", err
	}
	var digest [sha256.Size]byte
	h.Sum(digest[:0])
	return hex.EncodeToString(digest[:]), nil
}

// BenchmarkHashStackArray measures the proposed stack-array implementation.
func BenchmarkHashStackArray(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s, err := hashStackArray(benchMap)
		if err != nil {
			b.Fatal(err)
		}
		benchSinkString = s
	}
}

// BenchmarkSet measures the current Set() implementation on a 5-part key path.
// strings.Split allocates a []string and strings.Join is called on every loop iteration,
// even when no error occurs, producing N-1 discarded strings.
func BenchmarkSet(b *testing.B) {
	m := Map{
		"a": Map{
			"b": Map{
				"c": Map{
					"d": Map{
						"e": "original",
					},
				},
			},
		},
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := m.Set("a.b.c.d.e", "value"); err != nil {
			b.Fatal(err)
		}
	}
}

// setLazyPath is the proposed Set() implementation.
// parentPath is computed lazily — only in error branches — eliminating the
// per-iteration strings.Join allocations on the happy path.
func setLazyPath(m Map, keyPath string, value any) error {
	if m == nil {
		return nil
	}

	var curr any = m
	var parent any
	var key string
	var index uint
	var isIndex bool

	parts := strings.Split(keyPath, ".")
	for i, part := range parts {
		key = part
		parent = curr

		index, isIndex = parseIndex(part)

		if i == len(parts)-1 {
			if isIndex {
				sParent, ok := parent.([]any)
				if !ok {
					return fmt.Errorf("expected slice at %s, got %T", strings.Join(parts[:i], "."), parent)
				}
				if index >= uint(len(sParent)) {
					return fmt.Errorf("index out of bounds at %s: %d", strings.Join(parts[:i], "."), index)
				}
				sParent[index] = value
			} else {
				mParent, ok := isSMap(parent)
				if !ok {
					return fmt.Errorf("expected map at %s, got %T", strings.Join(parts[:i], "."), parent)
				}
				mParent[key] = value
			}
			return nil
		}

		if isIndex {
			sCurr, ok := curr.([]any)
			if !ok {
				return fmt.Errorf("expected slice at %s, got %T", strings.Join(parts[:i], "."), curr)
			}
			if index >= uint(len(sCurr)) {
				return fmt.Errorf("index out of bounds at %s: %d", strings.Join(parts[:i], "."), index)
			}
			curr = sCurr[index]
		} else {
			mCurr, ok := isSMap(curr)
			if !ok {
				return fmt.Errorf("expected map at %s, got %T", strings.Join(parts[:i], "."), curr)
			}
			curr = mCurr[key]
		}
	}
	return nil
}

// BenchmarkSetLazyPath measures the proposed lazy-parentPath implementation.
func BenchmarkSetLazyPath(b *testing.B) {
	m := Map{
		"a": Map{
			"b": Map{
				"c": Map{
					"d": Map{
						"e": "original",
					},
				},
			},
		},
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := setLazyPath(m, "a.b.c.d.e", "value"); err != nil {
			b.Fatal(err)
		}
	}
}
