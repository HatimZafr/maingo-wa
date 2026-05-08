package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Store struct {
	db         *sql.DB
	maxHistory int
	mu         sync.Mutex
}

func NewStore(dbPath string, maxHistory int) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open session db: %w", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		phone_number TEXT PRIMARY KEY,
		messages JSON NOT NULL DEFAULT '[]',
		updated_at INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		return nil, fmt.Errorf("create sessions table: %w", err)
	}

	return &Store{db: db, maxHistory: maxHistory}, nil
}

func (s *Store) Load(phone string) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var raw string
	err := s.db.QueryRow("SELECT messages FROM sessions WHERE phone_number = ?", phone).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}

	var msgs []Message
	if err := json.Unmarshal([]byte(raw), &msgs); err != nil {
		return nil, fmt.Errorf("unmarshal messages: %w", err)
	}
	return msgs, nil
}

func (s *Store) Save(phone string, messages []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(messages) > s.maxHistory {
		messages = messages[len(messages)-s.maxHistory:]
	}

	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO sessions (phone_number, messages, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(phone_number) DO UPDATE SET messages = ?, updated_at = ?`,
		phone, string(data), time.Now().Unix(), string(data), time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
