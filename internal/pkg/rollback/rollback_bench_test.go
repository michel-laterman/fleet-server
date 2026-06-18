// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package rollback

import (
	"context"
	"io"
	"testing"

	"github.com/rs/zerolog"
)

// BenchmarkRollbackChildLogger benchmarks the current Rollback implementation.
// Each registered function creates a child logger via r.log.With().Str().Logger(),
// allocating a new zerolog context per iteration.
func BenchmarkRollbackChildLogger(b *testing.B) {
	log := zerolog.New(io.Discard)
	r := New(log)
	noop := func(_ context.Context) error { return nil }
	r.Register("fn1", noop)
	r.Register("fn2", noop)
	r.Register("fn3", noop)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		_ = r.Rollback(ctx)
	}
}

// rollbackInlineFields is the proposed implementation that attaches the function
// name directly to each log call instead of constructing a child logger per iteration.
func rollbackInlineFields(r *Rollback, ctx context.Context) (err error) {
	for _, rb := range r.rbi {
		r.log.Debug().Str("rollback_fn_name", rb.name).Msg("rollback function called")
		if rerr := rb.fn(ctx); rerr != nil {
			r.log.Error().Err(rerr).Str("rollback_fn_name", rb.name).Msgf("rollback function %q failed", rb.name)
			if err == nil {
				err = rerr
			}
		} else {
			r.log.Debug().Str("rollback_fn_name", rb.name).Msgf("rollback function %q succeeded", rb.name)
		}
	}
	return
}

// BenchmarkRollbackInlineFields benchmarks the proposed inline-field implementation.
// No child logger allocation per function — fields are attached directly to each log event.
func BenchmarkRollbackInlineFields(b *testing.B) {
	log := zerolog.New(io.Discard)
	r := New(log)
	noop := func(_ context.Context) error { return nil }
	r.Register("fn1", noop)
	r.Register("fn2", noop)
	r.Register("fn3", noop)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		_ = rollbackInlineFields(r, ctx)
	}
}
