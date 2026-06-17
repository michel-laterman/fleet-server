// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build !integration

package policy

import "testing"

func BenchmarkRevisionString(b *testing.B) {
	r := Revision{PolicyID: "63f4e6d0-9626-11eb-b486-6de1529a4151", RevisionIdx: 42}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = r.String()
	}
}

func BenchmarkRevisionFromString(b *testing.B) {
	s := "policy:63f4e6d0-9626-11eb-b486-6de1529a4151:42"
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = RevisionFromString(s)
	}
}
