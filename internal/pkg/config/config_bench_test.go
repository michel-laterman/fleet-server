// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package config

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

var (
	benchSinkStr   string
	benchSinkLevel zerolog.Level
)

const (
	benchHost = "127.0.0.1"
	benchPort = uint16(8220)
)

// BenchmarkBindAddressFmt benchmarks the current bindAddress using fmt.Sprintf.
// Boxes the uint16 port as any (1 alloc) and allocates the output string (1 alloc).
func BenchmarkBindAddressFmt(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = bindAddress(benchHost, benchPort)
	}
}

// bindAddressConcat is the proposed implementation using strconv.FormatUint + concat.
func bindAddressConcat(host string, port uint16) string {
	return host + ":" + strconv.FormatUint(uint64(port), 10)
}

// BenchmarkBindAddressConcat benchmarks the proposed concat+FormatUint implementation.
// Eliminates the fmt reflection overhead; still 2 allocs (port string + concat result).
func BenchmarkBindAddressConcat(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = bindAddressConcat(benchHost, benchPort)
	}
}

// bindAddressAppend is the proposed implementation using strconv.AppendUint into a
// pre-sized byte slice, yielding a single alloc for the final string conversion.
func bindAddressAppend(host string, port uint16) string {
	buf := make([]byte, 0, len(host)+1+5) // host + ":" + up to 5 digits
	buf = append(buf, host...)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(port), 10)
	return string(buf)
}

// BenchmarkBindAddressAppend benchmarks the proposed AppendUint implementation.
// Single alloc: the final string(buf) conversion; no intermediate port string.
func BenchmarkBindAddressAppend(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSinkStr = bindAddressAppend(benchHost, benchPort)
	}
}

// BenchmarkStrToLevelCurrent benchmarks the current strToLevel:
// strings.ToLower allocates when input is not already lowercase.
func BenchmarkStrToLevelCurrent(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		l, err := strToLevel("  DEBUG  ")
		if err != nil {
			b.Fatal(err)
		}
		benchSinkLevel = l
	}
}

// strToLevelEqualFold is the proposed implementation using strings.EqualFold,
// which avoids the ToLower allocation for non-lowercase inputs.
func strToLevelEqualFold(s string) (zerolog.Level, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.EqualFold(s, "trace"):
		return zerolog.TraceLevel, nil
	case strings.EqualFold(s, "debug"):
		return zerolog.DebugLevel, nil
	case strings.EqualFold(s, "info"):
		return zerolog.InfoLevel, nil
	case strings.EqualFold(s, "warn"), strings.EqualFold(s, "warning"):
		return zerolog.WarnLevel, nil
	case strings.EqualFold(s, "error"):
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.DebugLevel, fmt.Errorf("invalid log level; must be one of: trace, debug, info, warn, error")
	}
}

// BenchmarkStrToLevelEqualFold benchmarks the proposed EqualFold implementation.
// TrimSpace returns a substring (0 allocs); EqualFold compares without lowercasing.
func BenchmarkStrToLevelEqualFold(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		l, err := strToLevelEqualFold("  DEBUG  ")
		if err != nil {
			b.Fatal(err)
		}
		benchSinkLevel = l
	}
}
