// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sqn

import (
	"strconv"
	"testing"
)

var (
	benchSinkString string
	benchSinkBytes  []byte
)

// appendJSON is the proposed AppendJSON implementation.
// It writes the JSON-encoded SeqNo directly into the caller-supplied buffer,
// avoiding the intermediate strings.Builder and string allocation from JSONString().
func appendJSON(s SeqNo, dst []byte) []byte {
	if len(s) == 0 {
		return append(dst, '[', ']')
	}
	dst = append(dst, '[')
	dst = strconv.AppendInt(dst, s[0], 10)
	for i := 1; i < len(s); i++ {
		dst = append(dst, ',')
		dst = strconv.AppendInt(dst, s[i], 10)
	}
	return append(dst, ']')
}

// BenchmarkJSONString measures the current JSONString() implementation:
// a strings.Builder accumulates the digits and returns a newly allocated string.
func BenchmarkJSONString(b *testing.B) {
	s := SeqNo{12345, 67890}
	b.ReportAllocs()
	for b.Loop() {
		benchSinkString = s.JSONString()
	}
}

// BenchmarkAppendJSON measures the proposed AppendJSON approach:
// digits are written directly into a caller-supplied buffer with zero allocations
// when the buffer has sufficient capacity.
func BenchmarkAppendJSON(b *testing.B) {
	s := SeqNo{12345, 67890}
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	for b.Loop() {
		dst = appendJSON(s, dst[:0])
		benchSinkBytes = dst
	}
}
