// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build !integration

package cache

import (
	"hash"
	"hash/fnv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

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

// BenchmarkScopedKeyConcat benchmarks the current "prefix" + id string concatenation.
// This is the hot path in ValidAPIKey and SetAPIKey — one heap alloc per call.
func BenchmarkScopedKeyConcat(b *testing.B) {
	id := benchAPIKeyID
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = "api:" + id
	}
}

// keyBuilderPool is the proposed pooled strings.Builder for scoped key construction.
// The builder's internal buffer is reused across calls; b.String() still allocates 1 string.
var keyBuilderPool = sync.Pool{
	New: func() any {
		sb := new(strings.Builder)
		sb.Grow(64)
		return sb
	},
}

// BenchmarkScopedKeyPooledBuilder benchmarks the pooled strings.Builder approach.
// Reuses the builder's byte buffer, so no buffer reallocation after warmup.
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

// keyHasherPool is a pool of FNV-64a hashers used to compute a uint64 key
// from prefix+id without materialising the combined string at all.
var keyHasherPool = sync.Pool{
	New: func() any { return fnv.New64a() },
}

// unsafeBytes converts a string to []byte without allocation.
// The caller must not modify the returned slice.
func unsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BenchmarkScopedKeyHash benchmarks computing a uint64 hash of "api:"+id
// using a pooled FNV hasher — 0 string allocs for key construction.
func BenchmarkScopedKeyHash(b *testing.B) {
	id := benchAPIKeyID
	b.ReportAllocs()
	for b.Loop() {
		h := keyHasherPool.Get().(hash.Hash64)
		h.Reset()
		h.Write(unsafeBytes("api:"))
		h.Write(unsafeBytes(id))
		benchSinkU64 = h.Sum64()
		keyHasherPool.Put(h)
	}
}

// --- makeArtifactKey benchmarks ---

// BenchmarkArtifactKeyFmt benchmarks the current makeArtifactKey using fmt.Sprintf.
// fmt.Sprintf boxes the variadic arguments and uses reflection-based formatting.
func BenchmarkArtifactKeyFmt(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = makeArtifactKey(benchArtIdent, benchArtSHA2)
	}
}

// BenchmarkArtifactKeyConcat benchmarks the proposed plain string concat.
// Eliminates fmt overhead; single alloc for the result string.
func BenchmarkArtifactKeyConcat(b *testing.B) {
	ident, sha2 := benchArtIdent, benchArtSHA2
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = "artifact:" + ident + ":" + sha2
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

// BenchmarkValidAPIKeyMiss benchmarks the current ValidAPIKey on a cache miss.
// Key construction allocates 1 string ("api:" + key.ID) per call.
func BenchmarkValidAPIKeyMiss(b *testing.B) {
	c := newBenchCache(b)
	defer c.cache.Close()
	key := benchAPIKeyVal
	b.ReportAllocs()
	for b.Loop() {
		benchSinkBool = c.ValidAPIKey(key)
	}
}

// validAPIKeyHashKey is the proposed ValidAPIKey implementation that hashes the
// prefix+ID directly to a uint64, eliminating the "api:"+key.ID string allocation.
func validAPIKeyHashKey(c *CacheT, key APIKey) bool {
	c.mut.RLock()
	defer c.mut.RUnlock()

	h := keyHasherPool.Get().(hash.Hash64)
	h.Reset()
	h.Write(unsafeBytes("api:"))
	h.Write(unsafeBytes(key.ID))
	keyHash := h.Sum64()
	keyHasherPool.Put(h)

	v, ok := c.cache.Get(keyHash)
	if ok {
		switch v {
		case "":
			ok = false
		case key.Key:
			// valid
		default:
			ok = false
		}
	}
	return ok
}

// BenchmarkValidAPIKeyMissHashKey benchmarks the proposed uint64-keyed ValidAPIKey.
// Key construction: 0 string allocs — hash computed into uint64 via pooled FNV hasher.
func BenchmarkValidAPIKeyMissHashKey(b *testing.B) {
	c := newBenchCache(b)
	defer c.cache.Close()
	key := benchAPIKeyVal
	b.ReportAllocs()
	for b.Loop() {
		benchSinkBool = validAPIKeyHashKey(c, key)
	}
}
