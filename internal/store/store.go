package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const DefaultConversationTitle = "新对话"

type Store struct {
	db *sql.DB
}

type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	ModelID   string    `json:"model_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             int64     `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	ModelID        string    `json:"model_id"`
	ContentRaw     string    `json:"content_raw"`
	ContentHTML    string    `json:"content_html"`
	ThinkContent   string    `json:"think_content,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	st := &Store{db: db}
	if err := st.init(); err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init() error {
	statements := []string{
		`PRAGMA journal_mode=WAL;`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			title TEXT NOT NULL,
			model_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_user_updated ON conversations(user_id, updated_at DESC);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			role TEXT NOT NULL,
			model_id TEXT NOT NULL,
			content_raw TEXT NOT NULL,
			content_html TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation_created ON messages(conversation_id, created_at ASC, id ASC);`,
	}
	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateSession(userID string, expiresAt time.Time) (string, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	_, err = s.db.Exec(`INSERT INTO sessions(token, user_id, expires_at, created_at) VALUES(?, ?, ?, ?)`, token, userID, expiresAt.Unix(), now)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) GetSession(token string) (*Session, error) {
	row := s.db.QueryRow(`SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ?`, token)
	var session Session
	var expiresAt int64
	var createdAt int64
	if err := row.Scan(&session.Token, &session.UserID, &expiresAt, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	session.ExpiresAt = time.Unix(expiresAt, 0)
	session.CreatedAt = time.Unix(createdAt, 0)
	return &session, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Store) DeleteExpiredSessions(now time.Time) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now.Unix())
	return err
}

func (s *Store) CreateConversation(userID, modelID string) (*Conversation, error) {
	id, err := randomHex(12)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	conversation := &Conversation{
		ID:        id,
		UserID:    userID,
		Title:     DefaultConversationTitle,
		ModelID:   modelID,
		CreatedAt: time.Unix(now, 0),
		UpdatedAt: time.Unix(now, 0),
	}
	_, err = s.db.Exec(`INSERT INTO conversations(id, user_id, title, model_id, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`, conversation.ID, conversation.UserID, conversation.Title, conversation.ModelID, now, now)
	if err != nil {
		return nil, err
	}
	return conversation, nil
}

func (s *Store) GetConversation(id string) (*Conversation, error) {
	row := s.db.QueryRow(`SELECT id, user_id, title, model_id, created_at, updated_at FROM conversations WHERE id = ?`, id)
	conversation, err := scanConversation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return conversation, err
}

func (s *Store) ListConversations(userID string) ([]Conversation, error) {
	rows, err := s.db.Query(`SELECT id, user_id, title, model_id, created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY updated_at DESC, created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		conversation, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, *conversation)
	}
	return conversations, rows.Err()
}

func (s *Store) UpdateConversation(id string, title string, modelID string) error {
	_, err := s.db.Exec(`UPDATE conversations SET title = ?, model_id = ?, updated_at = ? WHERE id = ?`, title, modelID, time.Now().Unix(), id)
	return err
}

func (s *Store) UpdateConversationTitle(id, title string) error {
	_, err := s.db.Exec(`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`, title, time.Now().Unix(), id)
	return err
}

func (s *Store) DeleteConversation(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM conversations WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AddMessage(conversationID, role, modelID, raw, html string) (*Message, error) {
	now := time.Now().Unix()
	result, err := s.db.Exec(`INSERT INTO messages(conversation_id, role, model_id, content_raw, content_html, created_at) VALUES(?, ?, ?, ?, ?, ?)`, conversationID, role, modelID, raw, html, now)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID); err != nil {
		return nil, err
	}
	return &Message{
		ID:             id,
		ConversationID: conversationID,
		Role:           role,
		ModelID:        modelID,
		ContentRaw:     raw,
		ContentHTML:    html,
		CreatedAt:      time.Unix(now, 0),
	}, nil
}

func (s *Store) GetMessage(id int64) (*Message, error) {
	row := s.db.QueryRow(`SELECT id, conversation_id, role, model_id, content_raw, content_html, created_at FROM messages WHERE id = ?`, id)
	message, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return message, err
}

func (s *Store) DeleteMessage(id int64) error {
	conversationID, lookupErr := s.conversationIDForMessage(id)
	if lookupErr != nil {
		if errors.Is(lookupErr, sql.ErrNoRows) {
			return nil
		}
		return lookupErr
	}
	result, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	return s.touchConversation(conversationID)
}

func (s *Store) DeleteMessagesFrom(conversationID string, fromMessageID int64) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE conversation_id = ? AND id >= ?`, conversationID, fromMessageID)
	if err != nil {
		return err
	}
	return s.touchConversation(conversationID)
}

func (s *Store) ListMessages(conversationID string) ([]Message, error) {
	rows, err := s.db.Query(`SELECT id, conversation_id, role, model_id, content_raw, content_html, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC, id ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *message)
	}
	return messages, rows.Err()
}

func (s *Store) conversationIDForMessage(id int64) (string, error) {
	row := s.db.QueryRow(`SELECT conversation_id FROM messages WHERE id = ?`, id)
	var conversationID string
	if err := row.Scan(&conversationID); err != nil {
		return "", err
	}
	return conversationID, nil
}

func (s *Store) touchConversation(conversationID string) error {
	_, err := s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, time.Now().Unix(), conversationID)
	return err
}

func (s *Store) HasMessages(conversationID string) (bool, error) {
	row := s.db.QueryRow(`SELECT COUNT(1) FROM messages WHERE conversation_id = ?`, conversationID)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func scanConversation(scanner interface{ Scan(dest ...any) error }) (*Conversation, error) {
	var conversation Conversation
	var createdAt int64
	var updatedAt int64
	if err := scanner.Scan(&conversation.ID, &conversation.UserID, &conversation.Title, &conversation.ModelID, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	conversation.CreatedAt = time.Unix(createdAt, 0)
	conversation.UpdatedAt = time.Unix(updatedAt, 0)
	return &conversation, nil
}

func scanMessage(scanner interface{ Scan(dest ...any) error }) (*Message, error) {
	var message Message
	var createdAt int64
	if err := scanner.Scan(&message.ID, &message.ConversationID, &message.Role, &message.ModelID, &message.ContentRaw, &message.ContentHTML, &createdAt); err != nil {
		return nil, err
	}
	message.CreatedAt = time.Unix(createdAt, 0)
	return &message, nil
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("random bytes: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
