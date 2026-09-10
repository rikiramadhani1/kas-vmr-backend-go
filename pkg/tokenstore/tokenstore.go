package tokenstore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenTTL = 7 * 24 * time.Hour
	refreshPrefix   = "refresh_token:"
)

// Store mirrors the original `tokenStore` object, backed by Redis.
type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func key(userID, token string) string {
	return fmt.Sprintf("%s%s:%s", refreshPrefix, userID, token)
}

// Add stores a refresh token for userID with a 7-day TTL.
func (s *Store) Add(ctx context.Context, userID, token string) error {
	return s.rdb.Set(ctx, key(userID, token), "valid", refreshTokenTTL).Err()
}

// Remove deletes a single refresh token.
func (s *Store) Remove(ctx context.Context, userID, token string) error {
	return s.rdb.Del(ctx, key(userID, token)).Err()
}

// Has checks whether a refresh token is still valid (exists in Redis).
func (s *Store) Has(ctx context.Context, userID, token string) (bool, error) {
	n, err := s.rdb.Exists(ctx, key(userID, token)).Result()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// RevokeAll deletes every refresh token belonging to userID.
//
// The original Node.js implementation used Redis' `KEYS pattern*` command
// here, which does a full, blocking O(N) scan of the entire keyspace and
// can stall Redis under load in production. We use SCAN instead, which
// walks the keyspace incrementally without blocking other clients.
func (s *Store) RevokeAll(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("%s%s:*", refreshPrefix, userID)

	var cursor uint64
	var keysToDelete []string

	for {
		keys, nextCursor, err := s.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		keysToDelete = append(keysToDelete, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if len(keysToDelete) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keysToDelete...).Err()
}
