package session

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s, err := NewStore(":memory:", 50)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	msgs, err := s.Load("6281234567890")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected empty history, got %d", len(msgs))
	}
}

func TestSaveAndLoad(t *testing.T) {
	s, err := NewStore(":memory:", 50)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	phone := "6281234567890"
	msgs := []Message{
		{Role: "user", Content: "Halo"},
		{Role: "assistant", Content: "Halo juga!"},
	}

	if err := s.Save(phone, msgs); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load(phone)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[0].Content != "Halo" {
		t.Errorf("got %q", loaded[0].Content)
	}
}

func TestUpdateSession(t *testing.T) {
	s, err := NewStore(":memory:", 50)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	phone := "6281234567890"

	s.Save(phone, []Message{{Role: "user", Content: "Pertama"}})
	s.Save(phone, []Message{{Role: "user", Content: "Kedua"}})

	loaded, _ := s.Load(phone)
	if len(loaded) != 1 {
		t.Fatalf("expected 1 message after update, got %d", len(loaded))
	}
	if loaded[0].Content != "Kedua" {
		t.Errorf("got %q", loaded[0].Content)
	}
}

func TestMaxHistory(t *testing.T) {
	s, err := NewStore(":memory:", 3)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	phone := "6281234567890"
	msgs := []Message{
		{Role: "user", Content: "1"},
		{Role: "assistant", Content: "2"},
		{Role: "user", Content: "3"},
		{Role: "assistant", Content: "4"},
		{Role: "user", Content: "5"},
	}

	if err := s.Save(phone, msgs); err != nil {
		t.Fatal(err)
	}

	loaded, _ := s.Load(phone)
	if len(loaded) != 3 {
		t.Fatalf("expected 3 messages (max), got %d", len(loaded))
	}
	if loaded[0].Content != "3" {
		t.Errorf("oldest kept should be '3', got %q", loaded[0].Content)
	}
}
