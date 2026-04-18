package session

import (
	"errors"
	"strings"
	"time"

	"web-ai/internal/store"
)

const DefaultTTL = 24 * time.Hour

type Manager struct {
	store *store.Store
	ttl   time.Duration
}

func NewManager(st *store.Store, ttl time.Duration) *Manager {
	return &Manager{store: st, ttl: ttl}
}

func (m *Manager) Create(userID string) (string, time.Time, error) {
	if err := m.store.DeleteExpiredSessions(time.Now()); err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(m.ttl)
	token, err := m.store.CreateSession(userID, expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (m *Manager) Authenticate(header string) (string, error) {
	token, err := bearerToken(header)
	if err != nil {
		return "", err
	}
	session, err := m.store.GetSession(token)
	if err != nil {
		return "", err
	}
	if session == nil {
		return "", errors.New("invalid token")
	}
	if !session.ExpiresAt.After(time.Now()) {
		_ = m.store.DeleteSession(token)
		return "", errors.New("expired token")
	}
	return session.UserID, nil
}

func bearerToken(header string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}
