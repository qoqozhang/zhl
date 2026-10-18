package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type SessionData struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Store struct {
	redis     *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func NewStore(redisClient *redis.Client) *Store {
	return &Store{
		redis:     redisClient,
		keyPrefix: "iam:session:",
		ttl:       24 * time.Hour,
	}
}

func (s *Store) Create(ctx context.Context, data SessionData) (string, error) {
	sessionID := uuid.New().String()
	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal session data failed: %w", err)
	}
	if err := s.redis.Set(ctx, s.keyPrefix+sessionID, payload, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("store session failed: %w", err)
	}
	return sessionID, nil
}

func (s *Store) Get(ctx context.Context, sessionID string) (*SessionData, error) {
	payload, err := s.redis.Get(ctx, s.keyPrefix+sessionID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get session failed: %w", err)
	}
	var data SessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session data failed: %w", err)
	}
	return &data, nil
}

func (s *Store) Delete(ctx context.Context, sessionID string) error {
	return s.redis.Del(ctx, s.keyPrefix+sessionID).Err()
}

func (s *Store) Touch(ctx context.Context, sessionID string) error {
	return s.redis.Expire(ctx, s.keyPrefix+sessionID, s.ttl).Err()
}