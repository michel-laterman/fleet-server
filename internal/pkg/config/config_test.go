// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:build !integration

package config

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	testlog "github.com/elastic/fleet-server/v7/internal/pkg/testing/log"

	"github.com/gofrs/uuid"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/go-ucfg"
)

func TestConfig(t *testing.T) {
	testcases := map[string]struct {
		err string
		cfg *Config
	}{
		"basic": {
			cfg: &Config{
				Fleet: defaultFleet(),
				Output: Output{
					Elasticsearch: defaultElastic(),
				},
				Inputs: []Input{
					{
						Type:   "fleet-server",
						Server: defaultServer(),
						Cache:  defaultCache(),
						Monitor: Monitor{
							FetchSize:          defaultFetchSize,
							PollTimeout:        defaultPollTimeout,
							PolicyDebounceTime: defaultPolicyDebounceTime,
						},
					},
				},
				Logging: defaultLogging(),
				HTTP:    defaultHTTP(),
			},
		},
		"fleet-logging": {
			cfg: &Config{
				Fleet: Fleet{
					Agent: Agent{
						ID: "1e4954ce-af37-4731-9f4a-407b08e69e42",
						Logging: AgentLogging{
							Level: "error",
						},
					},
				},
				Output: Output{
					Elasticsearch: defaultElastic(),
				},
				Inputs: []Input{
					{
						Type:   "fleet-server",
						Server: defaultServer(),
						Cache:  defaultCache(),
						Monitor: Monitor{
							FetchSize:          defaultFetchSize,
							PollTimeout:        defaultPollTimeout,
							PolicyDebounceTime: defaultPolicyDebounceTime,
						},
					},
				},
				Logging: defaultLogging(),
				HTTP:    defaultHTTP(),
			},
		},
		"input": {
			cfg: &Config{
				Fleet: defaultFleet(),
				Output: Output{
					Elasticsearch: defaultElastic(),
				},
				Inputs: []Input{
					{
						Type:   "fleet-server",
						Server: defaultServer(),
						Cache:  defaultCache(),
						Monitor: Monitor{
							FetchSize:          defaultFetchSize,
							PollTimeout:        defaultPollTimeout,
							PolicyDebounceTime: defaultPolicyDebounceTime,
						},
					},
				},
				Logging: defaultLogging(),
				HTTP:    defaultHTTP(),
			},
		},
		"input-config": {
			cfg: &Config{
				Fleet: defaultFleet(),
				Output: Output{
					Elasticsearch: defaultElastic(),
				},
				Inputs: []Input{
					{
						Type: "fleet-server",
						Server: Server{
							Host:         "localhost",
							Port:         8888,
							InternalPort: 8221,
							Timeouts: ServerTimeouts{
								Read:             20 * time.Second,
								ReadHeader:       5 * time.Second,
								Idle:             30 * time.Second,
								Write:            5 * time.Second,
								CheckinTimestamp: 30 * time.Second,
								CheckinLongPoll:  5 * time.Minute,
								CheckinJitter:    30 * time.Second,
								CheckinMaxPoll:   10 * time.Minute,
								Drain:            10 * time.Second,
							},
							Profiler: ServerProfiler{
								Enabled: false,
								Bind:    "localhost:6060",
							},
							CompressionLevel:  1,
							CompressionThresh: 1024,
							Limits:            generateServerLimits(0),
							Bulk:              defaultServerBulk(),
							GC:                defaultServerGC(),
							PGP: PGP{
								UpstreamURL: defaultPGPUpstreamURL,
								Dir:         filepath.Join(retrieveExecutableDir(), defaultPGPDirectoryName),
							},
						},
						Cache: generateCache(0),
						Monitor: Monitor{
							FetchSize:          defaultFetchSize,
							PollTimeout:        defaultPollTimeout,
							PolicyDebounceTime: defaultPolicyDebounceTime,
						},
					},
				},
				Logging: defaultLogging(),
				HTTP:    defaultHTTP(),
			},
		},
		"bad-input": {
			err: "input type must be \"fleet-server\"",
		},
		"bad-input-many": {
			err: "only 1 fleet-server input can be defined",
		},
		"bad-logging": {
			err: "invalid log level; must be one of: trace, debug, info, warn, error",
		},
		"bad-output": {
			err: "can only contain elasticsearch key",
		},
	}

	for name, test := range testcases {
		t.Run(name, func(t *testing.T) {
			l := testlog.SetLogger(t)
			zerolog.DefaultContextLogger = &l
			path := filepath.Join("testdata", name+".yml")
			cfg, err := LoadFile(path)
			if test.err != "" {
				if err == nil {
					t.Error("no error was reported")
				} else {
					cfgErr := err.(ucfg.Error) //nolint:errcheck,errorlint // this is checked below, but the linter doesn't respect it.
					require.Equal(t, test.err, cfgErr.Reason().Error())
				}
			} else {
				require.NoError(t, err)
				err = cfg.LoadServerLimits()
				require.NoError(t, err)
				skipUnexported := cmpopts.IgnoreUnexported(Config{})
				if !assert.True(t, cmp.Equal(test.cfg, cfg, skipUnexported)) {
					diff := cmp.Diff(test.cfg, cfg, skipUnexported)
					if diff != "" {
						t.Errorf("%s mismatch (-want +got):\n%s", name, diff)
					}
				}
			}
		})
	}

	t.Run("config specifies agent count", func(t *testing.T) {
		l := testlog.SetLogger(t)
		zerolog.DefaultContextLogger = &l
		path := filepath.Join("testdata", "input-specify-agents.yml")
		cfg, err := LoadFile(path)
		t.Logf("cfg fileread: %+v", cfg.Inputs[0].Server.Limits)
		require.NoError(t, err)
		err = cfg.LoadServerLimits()
		require.NoError(t, err)
		t.Logf("cfg loaded: %+v", cfg.Inputs[0].Server.Limits)

		t.Log("Before expect")
		expected := Config{
			Fleet: defaultFleet(),
			Output: Output{
				Elasticsearch: defaultElastic(),
			},
			Inputs: []Input{
				{
					Type:   "fleet-server",
					Server: defaultServer(),
					Cache:  generateCache(2500),
					Monitor: Monitor{
						FetchSize:          defaultFetchSize,
						PollTimeout:        defaultPollTimeout,
						PolicyDebounceTime: defaultPolicyDebounceTime,
					},
				},
			},
			Logging: defaultLogging(),
			HTTP:    defaultHTTP(),
		}
		expected.Inputs[0].Server.Limits = generateServerLimits(2500)
		t.Log("After expect")
		assert.EqualExportedValues(t, expected, *cfg)

	})
}

func TestLoadStandaloneAgentMetadata(t *testing.T) {
	t.Run("generates agent id", func(t *testing.T) {
		cfg := &Config{}
		cfg.LoadStandaloneAgentMetadata()
		assert.Len(t, cfg.Fleet.Agent.ID, 36)
		_, err := uuid.FromString(cfg.Fleet.Agent.ID)
		assert.NoError(t, err)

		assert.NotEmpty(t, cfg.Fleet.Agent.Version)
	})
}

func TestLoadServerLimits(t *testing.T) {
	t.Run("empty loads limits", func(t *testing.T) {
		c := &Config{Inputs: []Input{{}}}
		err := c.LoadServerLimits()
		assert.NoError(t, err)
		assert.NotZero(t, c.Inputs[0].Server.Limits.CheckinLimit.MaxBody)
		assert.NotZero(t, c.Inputs[0].Cache.ActionTTL)
	})
	t.Run("agent count limits load", func(t *testing.T) {
		c := &Config{Inputs: []Input{{
			Server: Server{
				Limits: ServerLimits{
					MaxAgents: 2500,
				},
			},
		}}}
		err := c.LoadServerLimits()
		assert.NoError(t, err)
		assert.NotZero(t, c.Inputs[0].Server.Limits.CheckinLimit.MaxBody)
		assert.Equal(t, time.Millisecond*5, c.Inputs[0].Server.Limits.CheckinLimit.Interval)

	})
	t.Run("agent count limits load does not override", func(t *testing.T) {
		c := &Config{Inputs: []Input{{
			Server: Server{
				Limits: ServerLimits{
					MaxAgents: 2500,
					ActionLimit: Limit{
						Interval: time.Millisecond,
					},
				},
			},
		}}}
		err := c.LoadServerLimits()
		assert.NoError(t, err)
		assert.NotZero(t, c.Inputs[0].Server.Limits.CheckinLimit.MaxBody)
		assert.Equal(t, time.Millisecond, c.Inputs[0].Server.Limits.ActionLimit.Interval)

	})
	t.Run("existing values are not overridden", func(t *testing.T) {
		c := &Config{
			Inputs: []Input{{
				Server: Server{
					Limits: ServerLimits{
						CheckinLimit: Limit{
							MaxBody: 5 * defaultCheckinMaxBody,
						},
					},
				},
				Cache: Cache{
					ActionTTL: time.Minute,
				},
			}},
		}
		err := c.LoadServerLimits()
		assert.NoError(t, err)
		assert.Equal(t, int64(5*defaultCheckinMaxBody), c.Inputs[0].Server.Limits.CheckinLimit.MaxBody)
		assert.NotZero(t, c.Inputs[0].Server.Limits.CheckinLimit.Burst)
		assert.NotZero(t, c.Inputs[0].Cache.ActionTTL)
	})

}

// Stub out the defaults so that the above is easier to maintain

func defaultCache() Cache {
	var d Cache
	d.InitDefaults()
	d.LoadLimits(loadLimits(0))
	return d
}

func generateCache(maxAgents int) Cache {
	var d Cache
	d.LoadLimits(loadLimits(maxAgents))
	return d
}

func generateServerLimits(maxAgents int) ServerLimits {
	var d ServerLimits
	d.MaxAgents = maxAgents
	d.LoadLimits(loadLimits(maxAgents))
	return d
}

func defaultServerBulk() ServerBulk {
	var d ServerBulk
	d.InitDefaults()
	return d
}

func defaultServerGC() GC {
	var d GC
	d.InitDefaults()
	return d
}

func defaultLogging() Logging {
	var d Logging
	d.InitDefaults()
	return d
}

func defaultHTTP() HTTP {
	var d HTTP
	d.InitDefaults()
	return d
}

func defaultFleet() Fleet {
	return Fleet{
		Agent: Agent{
			ID:      "1e4954ce-af37-4731-9f4a-407b08e69e42",
			Logging: AgentLogging{},
		},
	}
}

func defaultElastic() Elasticsearch {
	return Elasticsearch{
		Protocol:         "http",
		ServiceToken:     "test-token",
		Hosts:            []string{"localhost:9200"},
		MaxRetries:       3,
		MaxConnPerHost:   128,
		MaxContentLength: 104857600,
		Timeout:          90 * time.Second,
	}
}

func defaultServer() Server {
	var d Server
	d.InitDefaults()
	d.Limits.LoadLimits(loadLimits(0))
	return d
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("ELASTICSEARCH_SERVICE_TOKEN", "test-val")
	_ = testlog.SetLogger(t)
	path := filepath.Join("..", "testing", "fleet-server-testing.yml")
	c, err := LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test-val", c.Output.Elasticsearch.ServiceToken)
}

func TestDeprecationWarnings(t *testing.T) {
	var logCount atomic.Uint64
	log := testlog.SetLogger(t)
	log = log.Hook(zerolog.HookFunc(func(_ *zerolog.Event, _ zerolog.Level, _ string) {
		logCount.Add(1)
	}))
	oldLog := zerolog.DefaultContextLogger
	t.Cleanup(func() {
		zerolog.DefaultContextLogger = oldLog
	})
	zerolog.DefaultContextLogger = &log

	path := filepath.Join("testdata", "deprecated-config-attributes.yml")
	_, err := LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), logCount.Load(), "Expected 3 log messages")
}
