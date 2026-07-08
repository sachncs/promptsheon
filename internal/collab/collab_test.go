package collab_test

import (
	"testing"

	"github.com/sachncs/promptsheon/internal/collab"
)

func TestCreateSession(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "Hello world")
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.PromptID != "prompt1" {
		t.Errorf("expected prompt ID prompt1, got %s", session.PromptID)
	}
	if session.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got %s", session.Content)
	}
	if session.Version != 0 {
		t.Errorf("expected version 0, got %d", session.Version)
	}
}

func TestGetSession(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "test")

	got, err := manager.GetSession(session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, got.ID)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	manager := collab.NewManager()

	_, err := manager.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestApplyChangeInsert(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "Hello world")

	change := &collab.Change{
		ID:       "change1",
		UserID:   "user1",
		Type:     "insert",
		Position: 5,
		Content:  " beautiful",
	}

	err := manager.ApplyChange(session.ID, change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _, _ := manager.GetSessionContent(session.ID)
	if got != "Hello beautiful world" {
		t.Errorf("expected 'Hello beautiful world', got %s", got)
	}
}

func TestApplyChangeDelete(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "Hello world")

	change := &collab.Change{
		ID:       "change1",
		UserID:   "user1",
		Type:     "delete",
		Position: 5,
		Length:   6,
	}

	err := manager.ApplyChange(session.ID, change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _, _ := manager.GetSessionContent(session.ID)
	if got != "Hello" {
		t.Errorf("expected 'Hello', got %s", got)
	}
}

func TestApplyChangeReplace(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "Hello world")

	change := &collab.Change{
		ID:       "change1",
		UserID:   "user1",
		Type:     "replace",
		Position: 0,
		Content:  "Hi",
		Length:   5,
	}

	err := manager.ApplyChange(session.ID, change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _, _ := manager.GetSessionContent(session.ID)
	if got != "Hi world" {
		t.Errorf("expected 'Hi world', got %s", got)
	}
}

func TestUpdateCursor(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "test")

	cursor := &collab.Cursor{
		UserID:   "user1",
		Position: 5,
		Color:    "#ff0000",
	}

	err := manager.UpdateCursor(session.ID, cursor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := manager.GetSession(session.ID)
	if got.Cursors["user1"] == nil {
		t.Error("expected cursor to be set")
	}
	if got.Cursors["user1"].Position != 5 {
		t.Errorf("expected cursor position 5, got %d", got.Cursors["user1"].Position)
	}
}

func TestGetChanges(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "test")

	// Apply some changes
	_ = manager.ApplyChange(session.ID, &collab.Change{
		Type:     "insert",
		Position: 0,
		Content:  "A",
	})
	_ = manager.ApplyChange(session.ID, &collab.Change{
		Type:     "insert",
		Position: 1,
		Content:  "B",
	})

	// Get changes since version 0
	changes := manager.GetChanges(session.ID, 0)
	if len(changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(changes))
	}

	// Get changes since version 1
	changes = manager.GetChanges(session.ID, 1)
	if len(changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(changes))
	}
}

func TestCloseSession(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "test")
	manager.CloseSession(session.ID)

	_, err := manager.GetSession(session.ID)
	if err == nil {
		t.Error("expected error after closing session")
	}
}

func TestGetActiveSessions(t *testing.T) {
	manager := collab.NewManager()

	manager.CreateSession("prompt1", "test1")
	manager.CreateSession("prompt2", "test2")

	sessions := manager.GetActiveSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager := collab.NewManager()

	session := manager.CreateSession("prompt1", "test")

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(_ int) {
			_ = manager.ApplyChange(session.ID, &collab.Change{
				Type:     "insert",
				Position: 4, // Insert at end
				Content:  "X",
			})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	got, _, _ := manager.GetSessionContent(session.ID)
	// Original "test" + 10 X's = 14 characters
	if len(got) != 14 {
		t.Errorf("expected length 14, got %d", len(got))
	}
}
