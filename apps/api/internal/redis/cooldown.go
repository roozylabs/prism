package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/utils"
)

type CooldownStore struct {
	rdb *goredis.Client
}

func NewCooldownStore(rdb *goredis.Client) *CooldownStore {
	return &CooldownStore{rdb: rdb}
}

func (s *CooldownStore) SetCooldown(ctx context.Context, credentialID string, seconds int) error {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	return s.rdb.Set(ctx, key, "1", time.Duration(seconds)*time.Second).Err()
}

func (s *CooldownStore) IsCoolingDown(ctx context.Context, credentialID string) (bool, error) {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	val, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

func (s *CooldownStore) GetCooldownTTL(ctx context.Context, credentialID string) (time.Duration, error) {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	return s.rdb.TTL(ctx, key).Result()
}

// CoolingSet reports which of the given credentials are cooling down, with a
// single MGET over their cooldown keys. Only cooling credentials are present
// in the result. A nil store returns an empty set.
func (s *CooldownStore) CoolingSet(ctx context.Context, credentialIDs []string) (map[string]bool, error) {
	cooling := make(map[string]bool)
	if s == nil || s.rdb == nil || len(credentialIDs) == 0 {
		return cooling, nil
	}
	keys := make([]string, len(credentialIDs))
	for i, id := range credentialIDs {
		keys[i] = fmt.Sprintf("credential:%s:cooldown", id)
	}
	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return cooling, err
	}
	for i, v := range vals {
		if v == "1" {
			cooling[credentialIDs[i]] = true
		}
	}
	return cooling, nil
}

func (s *CooldownStore) RemoveCooldown(ctx context.Context, credentialID string) error {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	return s.rdb.Del(ctx, key).Err()
}

func (s *CooldownStore) ClearCooldown(ctx context.Context, credentialID string) error {
	return s.RemoveCooldown(ctx, credentialID)
}

func (s *CooldownStore) SetAccessToken(ctx context.Context, credentialID, token string, ttlSeconds int) error {
	key := fmt.Sprintf("credential:%s:access_token", credentialID)
	return s.rdb.Set(ctx, key, token, time.Duration(ttlSeconds)*time.Second).Err()
}

func (s *CooldownStore) GetAccessToken(ctx context.Context, credentialID string) (string, error) {
	key := fmt.Sprintf("credential:%s:access_token", credentialID)
	return s.rdb.Get(ctx, key).Result()
}

func (s *CooldownStore) DeleteAccessToken(ctx context.Context, credentialID string) error {
	key := fmt.Sprintf("credential:%s:access_token", credentialID)
	return s.rdb.Del(ctx, key).Err()
}

type CredentialQuotaInfo struct {
	RemainingRequests int64  `json:"remainingRequests,omitempty"`
	LimitRequests     int64  `json:"limitRequests,omitempty"`
	RemainingTokens   int64  `json:"remainingTokens,omitempty"`
	LimitTokens       int64  `json:"limitTokens,omitempty"`
	ResetDurationSec  int    `json:"resetDurationSec,omitempty"`
	ResetAt           string `json:"resetAt,omitempty"`
	StatusText        string `json:"statusText,omitempty"`
	LastUpdated       int64  `json:"lastUpdated"`
}

func (s *CooldownStore) SaveCredentialQuota(ctx context.Context, credentialID string, quota *CredentialQuotaInfo) error {
	if credentialID == "" || quota == nil {
		return nil
	}
	quota.LastUpdated = time.Now().Unix()
	data, err := json.Marshal(quota)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("credential:%s:quota", credentialID)
	return s.rdb.Set(ctx, key, data, 24*time.Hour).Err()
}

func (s *CooldownStore) GetCredentialQuota(ctx context.Context, credentialID string) (*CredentialQuotaInfo, error) {
	key := fmt.Sprintf("credential:%s:quota", credentialID)
	data, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var quota CredentialQuotaInfo
	if err := json.Unmarshal([]byte(data), &quota); err != nil {
		return nil, err
	}
	return &quota, nil
}

func (s *CooldownStore) DeleteCredentialQuota(ctx context.Context, credentialID string) error {
	key := fmt.Sprintf("credential:%s:quota", credentialID)
	return s.rdb.Del(ctx, key).Err()
}

const ActiveRequestsHashKey = "gateway:active_requests"

type ActiveRequestRecord struct {
	UserID       string `json:"userId"`
	Model        string `json:"model"`
	GatewayKeyID string `json:"gatewayKeyId"`
	Credential   string `json:"credential"`
	StartedAt    int64  `json:"startedAt"`
}

type ActiveStreamsSummary struct {
	TotalActive  int64            `json:"totalActive"`
	ByModel      map[string]int64 `json:"byModel"`
	ByCredential map[string]int64 `json:"byCredential"`
	ByKey        map[string]int64 `json:"byKey"`
}

func (s *CooldownStore) TrackActiveStream(ctx context.Context, reqID, userID, modelSlug, gatewayKeyID, credName string) error {
	if reqID == "" {
		return nil
	}
	rec := ActiveRequestRecord{
		UserID:       userID,
		Model:        modelSlug,
		GatewayKeyID: gatewayKeyID,
		Credential:   utils.MaskEmailName(credName),
		StartedAt:    time.Now().Unix(),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.rdb.HSet(ctx, ActiveRequestsHashKey, reqID, data).Err()
}

func (s *CooldownStore) UntrackActiveStream(ctx context.Context, reqID string) error {
	if reqID == "" {
		return nil
	}
	return s.rdb.HDel(ctx, ActiveRequestsHashKey, reqID).Err()
}

func (s *CooldownStore) IncrementActiveStream(ctx context.Context, modelSlug, gatewayKeyID, credName string) error {
	return nil
}

func (s *CooldownStore) DecrementActiveStream(ctx context.Context, modelSlug, gatewayKeyID, credName string) error {
	return nil
}

// GetActiveStreams summarizes in-flight requests. With a userID it counts only
// that user's requests; an empty userID counts every request, for checks that
// must see all traffic, such as refusing to delete a credential in use.
func (s *CooldownStore) GetActiveStreams(ctx context.Context, userID string) (*ActiveStreamsSummary, error) {
	summary := &ActiveStreamsSummary{
		TotalActive:  0,
		ByModel:      make(map[string]int64),
		ByCredential: make(map[string]int64),
		ByKey:        make(map[string]int64),
	}

	items, err := s.rdb.HGetAll(ctx, ActiveRequestsHashKey).Result()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return summary, err
	}

	now := time.Now().Unix()
	var staleKeys []string

	for reqID, rawJSON := range items {
		var rec ActiveRequestRecord
		if err := json.Unmarshal([]byte(rawJSON), &rec); err != nil {
			staleKeys = append(staleKeys, reqID)
			continue
		}

		// Clean up any requests older than 2 minutes (120s) as stale
		if rec.StartedAt > 0 && (now-rec.StartedAt) > 120 {
			staleKeys = append(staleKeys, reqID)
			continue
		}
		if userID != "" && rec.UserID != userID {
			continue
		}

		summary.TotalActive++
		if rec.Model != "" {
			summary.ByModel[rec.Model]++
		}
		if rec.Credential != "" {
			summary.ByCredential[rec.Credential]++
		}
		if rec.GatewayKeyID != "" {
			summary.ByKey[rec.GatewayKeyID]++
		}
	}

	if len(staleKeys) > 0 {
		_ = s.rdb.HDel(ctx, ActiveRequestsHashKey, staleKeys...).Err()
	}

	return summary, nil
}

// Circuit Breaker Methods

const (
	CircuitBreakerThreshold = 3  // 3 consecutive 50x/timeout errors
	CircuitBreakerTTLSec    = 60 // 60 seconds quarantine
)

func (s *CooldownStore) RecordServerError(ctx context.Context, credentialID string, statusCode int) (bool, error) {
	if s == nil || s.rdb == nil || credentialID == "" {
		return false, nil
	}

	key := fmt.Sprintf("credential:%s:50x_count", credentialID)
	count, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		_ = s.rdb.Expire(ctx, key, time.Duration(CircuitBreakerTTLSec)*time.Second).Err()
	}

	if count >= int64(CircuitBreakerThreshold) {
		// Put credential into quarantine cooldown for 60 seconds
		_ = s.SetCooldown(ctx, credentialID, CircuitBreakerTTLSec)
		_ = s.rdb.Del(ctx, key).Err()

		// Update quota status to note circuit breaker
		_ = s.SaveCredentialQuota(ctx, credentialID, &CredentialQuotaInfo{
			StatusText:  "circuit_breaker_50x",
			LastUpdated: time.Now().Unix(),
		})

		return true, nil
	}

	return false, nil
}

func (s *CooldownStore) RecordSuccess(ctx context.Context, credentialID string) error {
	if s == nil || s.rdb == nil || credentialID == "" {
		return nil
	}
	key := fmt.Sprintf("credential:%s:50x_count", credentialID)
	return s.rdb.Del(ctx, key).Err()
}
