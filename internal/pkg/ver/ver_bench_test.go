// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package ver

import (
	"strconv"
	"strings"
	"testing"

	goversion "github.com/hashicorp/go-version"
)

var benchSinkStr string

var benchVer = goversion.Must(goversion.NewVersion("8.12.1"))

// BenchmarkMinimizePatch benchmarks the current minimizePatch implementation:
// allocates a []string, fills it with strconv.Itoa results, then strings.Join.
func BenchmarkMinimizePatch(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = minimizePatch(benchVer)
	}
}

// minimizePatchBuilder is the proposed implementation using strings.Builder,
// writing digits directly without the intermediate []string allocation.
func minimizePatchBuilder(ver *goversion.Version) string {
	segments := ver.Segments()
	if len(segments) > 2 {
		segments = segments[:2]
	}
	var b strings.Builder
	b.Grow(16)
	for i, seg := range segments {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(strconv.Itoa(seg))
	}
	b.WriteString(".0")
	return b.String()
}

// BenchmarkMinimizePatchBuilder benchmarks the proposed strings.Builder implementation.
func BenchmarkMinimizePatchBuilder(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = minimizePatchBuilder(benchVer)
	}
}

// BenchmarkBuildVersionConstraintFmt benchmarks the current buildVersionConstraint:
// fmt.Sprintf(">= %s", minimizePatch(ver)) wraps minimizePatch with an extra fmt alloc.
func BenchmarkBuildVersionConstraintFmt(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := buildVersionConstraint("8.12.1")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// buildVersionConstraintConcat is the proposed implementation replacing fmt.Sprintf
// with string concat, avoiding the format interpreter overhead.
func buildVersionConstraintConcat(fleetVersion string) (any, error) {
	ver, err := parseVersion(fleetVersion)
	if err != nil {
		return nil, err
	}
	return goversion.NewConstraint(">= " + minimizePatchBuilder(ver))
}

// BenchmarkBuildVersionConstraintConcat benchmarks the proposed concat implementation.
func BenchmarkBuildVersionConstraintConcat(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := buildVersionConstraintConcat("8.12.1")
		if err != nil {
			b.Fatal(err)
		}
	}
}
