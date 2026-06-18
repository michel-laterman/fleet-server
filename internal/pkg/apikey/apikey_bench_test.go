// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build !integration

package apikey

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"
	"unicode/utf8"
)

var (
	benchSinkAPIKey *APIKey
	benchSinkString string
)

// Realistic API key ID and key lengths matching Elasticsearch production output.
const (
	benchKeyID  = "mDf80YoBkMO6C2IiUDOP"
	benchKeyKey = "neQRplXBQneSTS0WqmiUEg"
)

var benchToken = base64.StdEncoding.EncodeToString([]byte(benchKeyID + ":" + benchKeyKey))

// BenchmarkNewAPIKeyFromToken measures the current implementation:
// base64.DecodeString + string([]byte) copy + strings.Split allocating []string.
func BenchmarkNewAPIKeyFromToken(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		k, err := NewAPIKeyFromToken(benchToken)
		if err != nil {
			b.Fatal(err)
		}
		benchSinkAPIKey = k
	}
}

// newAPIKeyBytesCut is the proposed implementation using bytes.Cut.
// It eliminates the intermediate string(d) copy and the []string from strings.Split.
func newAPIKeyBytesCut(token string) (*APIKey, error) {
	d, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !utf8.Valid(d) {
		return nil, ErrInvalidToken
	}
	id, key, ok := bytes.Cut(d, []byte(":"))
	if !ok {
		return nil, ErrMalformedToken
	}
	return &APIKey{ID: string(id), Key: string(key)}, nil
}

// BenchmarkNewAPIKeyFromTokenBytesCut measures the proposed bytes.Cut implementation.
func BenchmarkNewAPIKeyFromTokenBytesCut(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		k, err := newAPIKeyBytesCut(benchToken)
		if err != nil {
			b.Fatal(err)
		}
		benchSinkAPIKey = k
	}
}

// BenchmarkToken measures the current Token() implementation:
// fmt.Sprintf builds "id:key", then []byte cast, then base64 encode — three allocations.
func BenchmarkToken(b *testing.B) {
	k := APIKey{ID: benchKeyID, Key: benchKeyKey}
	b.ReportAllocs()
	for b.Loop() {
		benchSinkString = k.Token()
	}
}

// BenchmarkTokenStringConcat measures the proposed Token() implementation:
// string concat for "id:key" replaces fmt.Sprintf, then []byte cast, then base64 encode.
func BenchmarkTokenStringConcat(b *testing.B) {
	k := APIKey{ID: benchKeyID, Key: benchKeyKey}
	b.ReportAllocs()
	for b.Loop() {
		benchSinkString = base64.StdEncoding.EncodeToString([]byte(k.ID + ":" + k.Key))
	}
}

// BenchmarkBuildAuthHeader measures the current Authenticate() header-value construction:
// k.Token() (fmt.Sprintf + []byte + base64) then a second fmt.Sprintf to prepend "ApiKey ".
func BenchmarkBuildAuthHeader(b *testing.B) {
	k := APIKey{ID: benchKeyID, Key: benchKeyKey}
	b.ReportAllocs()
	for b.Loop() {
		benchSinkString = fmt.Sprintf("%s%s", authPrefix, k.Token())
	}
}

// BenchmarkBuildAuthHeaderDirect measures the proposed approach:
// one string concat for "id:key", one base64 encode, one prefix concat — two allocations total.
func BenchmarkBuildAuthHeaderDirect(b *testing.B) {
	k := APIKey{ID: benchKeyID, Key: benchKeyKey}
	b.ReportAllocs()
	for b.Loop() {
		benchSinkString = authPrefix + base64.StdEncoding.EncodeToString([]byte(k.ID+":"+k.Key))
	}
}
