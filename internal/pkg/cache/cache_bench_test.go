// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build !integration

package cache

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elastic/fleet-server/v7/internal/pkg/config"
)

const (
	benchAPIKeyID  = "mDf80YoBkMO6C2IiUDOP"
	benchAPIKeyKey = "neQRplXBQneSTS0WqmiUEg"
	benchArtIdent  = "endpoint-8.12.0-linux-x86_64"
	benchArtSHA2   = "sha256:deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
)

var (
	benchAPIKeyVal = APIKey{ID: benchAPIKeyID, Key: benchAPIKeyKey}
	benchSinkStr   string
	benchSinkU64   uint64
	benchSinkBool  bool
)

// --- Scoped key construction benchmarks (isolated, no Ristretto) ---

// BenchmarkScopedKeyConcat benchmarks the old "prefix" + id string concatenation baseline.
func BenchmarkScopedKeyConcat(b *testing.B) {
	id := benchAPIKeyID
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = "api:" + id
	}
}

// keyBuilderPool shows the pooled strings.Builder approach, retained for comparison.
// b.String() still allocates 1 string — measured to be worse than concat.
var keyBuilderPool = sync.Pool{
	New: func() any {
		sb := new(strings.Builder)
		sb.Grow(64)
		return sb
	},
}

// BenchmarkScopedKeyPooledBuilder benchmarks the pooled strings.Builder approach.
func BenchmarkScopedKeyPooledBuilder(b *testing.B) {
	id := benchAPIKeyID
	b.ReportAllocs()
	for b.Loop() {
		sb := keyBuilderPool.Get().(*strings.Builder)
		sb.Reset()
		sb.WriteString("api:")
		sb.WriteString(id)
		benchSinkStr = sb.String()
		keyBuilderPool.Put(sb)
	}
}

// BenchmarkScopedKeyHash benchmarks the production uint64 hash path — 0 allocs.
func BenchmarkScopedKeyHash(b *testing.B) {
	id := benchAPIKeyID
	b.ReportAllocs()
	for b.Loop() {
		benchSinkU64 = scopedKeyHash("api:", id)
	}
}

// --- makeArtifactKey benchmarks ---

// BenchmarkArtifactKeyConcat benchmarks the old plain string concat baseline.
func BenchmarkArtifactKeyConcat(b *testing.B) {
	ident, sha2 := benchArtIdent, benchArtSHA2
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = "artifact:" + ident + ":" + sha2
	}
}

// BenchmarkArtifactKeyHash benchmarks the production uint64 hash path — 0 allocs.
func BenchmarkArtifactKeyHash(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSinkU64 = artifactKeyHash(benchArtIdent, benchArtSHA2)
	}
}

// --- Full ValidAPIKey operation benchmarks (cache miss) ---

func newBenchCache(b *testing.B) *CacheT {
	b.Helper()
	c, err := New(config.Cache{
		NumCounters: 1000,
		MaxCost:     1 << 20,
		APIKeyTTL:   5 * time.Minute,
	})
	if err != nil {
		b.Fatal(err)
	}
	return c
}

// BenchmarkValidAPIKeyMiss benchmarks ValidAPIKey on a cache miss.
// Key construction uses a pooled FNV hasher — 0 string allocs.
func BenchmarkValidAPIKeyMiss(b *testing.B) {
	c := newBenchCache(b)
	defer c.cache.Close()
	key := benchAPIKeyVal
	b.ReportAllocs()
	for b.Loop() {
		benchSinkBool = c.ValidAPIKey(key)
	}
}
