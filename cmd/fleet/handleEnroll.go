// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/elastic/fleet-server/v7/internal/pkg/apikey"
	"github.com/elastic/fleet-server/v7/internal/pkg/bulk"
	"github.com/elastic/fleet-server/v7/internal/pkg/cache"
	"github.com/elastic/fleet-server/v7/internal/pkg/config"
	"github.com/elastic/fleet-server/v7/internal/pkg/dl"
	"github.com/elastic/fleet-server/v7/internal/pkg/limit"
	"github.com/elastic/fleet-server/v7/internal/pkg/model"
	"github.com/elastic/fleet-server/v7/internal/pkg/sqn"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gofrs/uuid"
	"github.com/hashicorp/go-version"
	"github.com/julienschmidt/httprouter"
	"github.com/miolini/datacounter"
	"github.com/rs/zerolog/log"
)

const (
	kEnrollMod = "enroll"

	kCacheAccessInitTTL = time.Second * 30 // Cache a bit longer to handle expensive initial checkin
	kCacheEnrollmentTTL = time.Second * 30
)

var (
	ErrUnknownEnrollType = errors.New("unknown enroll request type")
)

type EnrollerT struct {
	verCon version.Constraints
	bulker bulk.Bulk
	cache  cache.Cache
	limit  *limit.Limiter
}

func NewEnrollerT(verCon version.Constraints, cfg *config.Server, bulker bulk.Bulk, c cache.Cache) (*EnrollerT, error) {

	log.Info().
		Interface("limits", cfg.Limits.EnrollLimit).
		Msg("Enroller install limits")

	return &EnrollerT{
		verCon: verCon,
		limit:  limit.NewLimiter(&cfg.Limits.EnrollLimit),
		bulker: bulker,
		cache:  c,
	}, nil

}

func (rt Router) handleEnroll(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	start := time.Now()

	// Work around wonky router rule
	if ps.ByName("id") != "enroll" {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	data, err := rt.et.handleEnroll(r)

	if err != nil {
		code, str, msg, lvl := cntEnroll.IncError(err)

		log.WithLevel(lvl).
			Err(err).
			Str("mod", kEnrollMod).
			Int("code", code).
			Dur("tdiff", time.Since(start)).
			Msg("Enroll fail")

		if err := WriteError(w, code, str, msg); err != nil {
			log.Error().Err(err).Msg("fail writing error response")
		}
		return
	}

	var numWritten int
	if numWritten, err = w.Write(data); err != nil {
		log.Error().Err(err).Msg("fail send enroll response")
	}

	cntEnroll.bodyOut.Add(uint64(numWritten))

	log.Trace().
		Err(err).
		RawJSON("raw", data).
		Str("mod", kEnrollMod).
		Dur("rtt", time.Since(start)).
		Msg("handleEnroll OK")
}

func (et *EnrollerT) handleEnroll(r *http.Request) ([]byte, error) {

	limitF, err := et.limit.Acquire()
	if err != nil {
		return nil, err
	}
	defer limitF()

	key, err := authApiKey(r, et.bulker.Client(), et.cache)
	if err != nil {
		return nil, err
	}

	err = validateUserAgent(r, et.verCon)
	if err != nil {
		return nil, err
	}

	// Metrics; serenity now.
	dfunc := cntEnroll.IncStart()
	defer dfunc()

	// Validate that an enrollment record exists for a key with this id.
	erec, err := et.fetchEnrollmentKeyRecord(r.Context(), key.Id)
	if err != nil {
		return nil, err
	}

	readCounter := datacounter.NewReaderCounter(r.Body)

	// Parse the request body
	req, err := decodeEnrollRequest(readCounter)
	if err != nil {
		return nil, err
	}

	cntEnroll.bodyIn.Add(readCounter.Count())

	resp, err := _enroll(r.Context(), et.bulker, et.cache, *req, *erec)
	if err != nil {
		return nil, err
	}

	return json.Marshal(resp)
}

func _enroll(ctx context.Context, bulker bulk.Bulk, c cache.Cache, req EnrollRequest, erec model.EnrollmentApiKey) (*EnrollResponse, error) {

	if req.SharedId != "" {
		// TODO: Support pre-existing install
		return nil, errors.New("preexisting install not yet supported")
	}

	now := time.Now()

	// Generate an ID here so we can pre-create the api key and avoid a round trip
	u, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	// TODO: Cleanup after ourselves on failure:
	// Revoke generated keys.
	// Remove agent record.

	agentId := u.String()

	accessApiKey, err := generateAccessApiKey(ctx, bulker.Client(), agentId)
	if err != nil {
		return nil, err
	}

	// Update the local metadata agent id
	localMeta, err := updateLocalMetaAgentId(req.Meta.Local, agentId)
	if err != nil {
		return nil, err
	}

	agentData := model.Agent{
		Active:         true,
		PolicyId:       erec.PolicyId,
		Type:           req.Type,
		EnrolledAt:     now.UTC().Format(time.RFC3339),
		LocalMetadata:  localMeta,
		AccessApiKeyId: accessApiKey.Id,
		ActionSeqNo:    []int64{sqn.UndefinedSeqNo},
	}

	err = createFleetAgent(ctx, bulker, agentId, agentData)
	if err != nil {
		return nil, err
	}

	resp := EnrollResponse{
		Action: "created",
		Item: EnrollResponseItem{
			ID:             agentId,
			Active:         agentData.Active,
			PolicyId:       agentData.PolicyId,
			Type:           agentData.Type,
			EnrolledAt:     agentData.EnrolledAt,
			UserMeta:       agentData.UserProvidedMetadata,
			LocalMeta:      agentData.LocalMetadata,
			AccessApiKeyId: agentData.AccessApiKeyId,
			AccessAPIKey:   accessApiKey.Token(),
			Status:         "online",
		},
	}

	// We are Kool & and the Gang; cache the access key to avoid the roundtrip on impending checkin
	c.SetApiKey(*accessApiKey, kCacheAccessInitTTL)

	return &resp, nil
}

// updateMetaLocalAgentId updates the agent id in the local metadata if exists
// At the time of writing the local metadata blob looks something like this
// {
//     "elastic": {
//         "agent": {
//             "id": "1b9c327a-c93a-4aef-b67f-effbef54d836",
//             "version": "8.0.0",
//             "snapshot": false,
//             "upgradeable": false
//         }
//     },
//     "host": {
//         "architecture": "x86_64",
//         "hostname": "eh-Hounddiamond",
//         "name": "eh-Hounddiamond",
//         "id": "1b9c327a-c93a-4aef-b67f-effbef54d836"
//     },
//     "os": {
//         "family": "darwin",
//         "kernel": "19.6.0",
//         "platform": "darwin",
//         "version": "10.15.7",
//         "name": "Mac OS X",
//         "full": "Mac OS X(10.15.7)"
//     }
// }
func updateLocalMetaAgentId(data []byte, agentId string) ([]byte, error) {
	if data == nil {
		return data, nil
	}

	var m map[string]interface{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	if v, ok := m["elastic"]; ok {
		if sm, ok := v.(map[string]interface{}); ok {
			if v, ok = sm["agent"]; ok {
				if sm, ok = v.(map[string]interface{}); ok {
					if _, ok = sm["id"]; ok {
						sm["id"] = agentId
						data, err = json.Marshal(m)
						if err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}

	return data, nil
}

func createFleetAgent(ctx context.Context, bulker bulk.Bulk, id string, agent model.Agent) error {
	data, err := json.Marshal(agent)
	if err != nil {
		return err
	}

	_, err = bulker.Create(ctx, dl.FleetAgents, id, data, bulk.WithRefresh())
	if err != nil {
		return err
	}
	return nil
}

func generateAccessApiKey(ctx context.Context, client *elasticsearch.Client, agentId string) (*apikey.ApiKey, error) {
	return apikey.Create(ctx, client, agentId, "", []byte(kFleetAccessRolesJSON),
		apikey.NewMetadata(agentId, apikey.TypeAccess))
}

func generateOutputApiKey(ctx context.Context, client *elasticsearch.Client, agentId, outputName string, roles []byte) (*apikey.ApiKey, error) {
	name := fmt.Sprintf("%s:%s", agentId, outputName)
	return apikey.Create(ctx, client, name, "", roles,
		apikey.NewMetadata(agentId, apikey.TypeOutput))
}

func (et *EnrollerT) fetchEnrollmentKeyRecord(ctx context.Context, id string) (*model.EnrollmentApiKey, error) {

	if key, ok := et.cache.GetEnrollmentApiKey(id); ok {
		return &key, nil
	}

	// Pull API key record from .fleet-enrollment-api-keys
	rec, err := dl.FindEnrollmentAPIKey(ctx, et.bulker, dl.QueryEnrollmentAPIKeyByID, dl.FieldApiKeyID, id)
	if err != nil {
		return nil, err
	}

	if !rec.Active {
		return nil, fmt.Errorf("record is inactive")
	}

	cost := int64(len(rec.ApiKey))
	et.cache.SetEnrollmentApiKey(id, rec, cost, kCacheEnrollmentTTL)

	return &rec, nil
}

func decodeEnrollRequest(data io.Reader) (*EnrollRequest, error) {

	// TODO: defend overflow, slow roll
	var req EnrollRequest
	decoder := json.NewDecoder(data)
	if err := decoder.Decode(&req); err != nil {
		return nil, err
	}

	// Validate
	switch req.Type {
	// TODO: Should these be converted to constant? Need to be kept in sync with Kibana?
	case "EPHEMERAL", "PERMANENT", "TEMPORARY":
	default:
		return nil, ErrUnknownEnrollType
	}

	return &req, nil
}
