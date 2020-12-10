// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package es

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Error
type ErrorT struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
	Cause  struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	} `json:"caused_by"`
}

// Acknowledgement response
type AckResponse struct {
	Acknowledged bool   `json:"acknowledged"`
	Error        ErrorT `json:"error,omitempty"`
}

type HitT struct {
	Id     string          `json:"_id"`
	SeqNo  int64           `json:"_seq_no"`
	Index  string          `json:"_index"`
	Source json.RawMessage `json:"_source"`
	Score  *float64        `json:"_score"`
}

type HitsT struct {
	Hits  []HitT `json:"hits"`
	Total struct {
		Relation string `json:"relation"`
		Value    uint64 `json:"value"`
	} `json:"total"`
	MaxScore *float64 `json:"max_score"`
}

type Bucket struct {
	Key          string           `json:"key"`
	DocCount     int64            `json:"doc_count"`
	Aggregations map[string]HitsT `json:"-"`
}

type _bucket Bucket

func (b *Bucket) UnmarshalJSON(data []byte) error {
	b2 := _bucket{}
	err := json.Unmarshal(data, &b2)
	if err != nil {
		return err
	}
	var aggs map[string]interface{}
	err = json.Unmarshal(data, &aggs)
	if err != nil {
		return err
	}
	typ := reflect.TypeOf(b2)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag != "" && jsonTag != "-" {
			delete(aggs, jsonTag)
		}
	}
	b2.Aggregations = make(map[string]HitsT)
	for name, value := range aggs {
		vMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		hMap, ok := vMap["hits"]
		if !ok {
			continue
		}
		data, err := json.Marshal(hMap)
		if err != nil {
			return err
		}
		var hits HitsT
		err = json.Unmarshal(data, &hits)
		if err != nil {
			return err
		}
		b2.Aggregations[name] = hits
	}
	*b = Bucket(b2)
	return nil
}

type Aggregation struct {
	Value                   float64  `json:"value"`
	DocCountErrorUpperBound int64    `json:"doc_count_error_upper_bound"`
	SumOtherDocCount        int64    `json:"sum_other_doc_count"`
	Buckets                 []Bucket `json:"buckets,omitempty"`
}

type Response struct {
	Status   int    `json:"status"`
	Took     uint64 `json:"took"`
	TimedOut bool   `json:"timed_out"`
	Shards   struct {
		Total      uint64 `json:"total"`
		Successful uint64 `json:"successful"`
		Skipped    uint64 `json:"skipped"`
		Failed     uint64 `json:"failed"`
	} `json:"_shards"`
	Hits         HitsT                  `json:"hits"`
	Aggregations map[string]Aggregation `json:"aggregations,omitempty"`

	Error ErrorT `json:"error,omitempty"`
}

type ResultT struct {
	HitsT
	Aggregations map[string]Aggregation
}
