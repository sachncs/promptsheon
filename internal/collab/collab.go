// Package collab provides real-time collaborative editing capabilities.
package collab

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Cursor represents a user's cursor position.
type Cursor struct {
	UserID    string    `json:"user_id"`
	Position  int       `json:"position"`
	Selection *Range    `json:"selection,omitempty"`
	Color     string    `json:"color"`
	Timestamp time.Time `json:"timestamp"`
}

// Range represents a text selection range.
type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Change represents a text change.
type Change struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"` // "insert", "delete", "replace"
	Position  int       `json:"position"`
	Content   string    `json:"content,omitempty"`
	Length    int       `json:"length,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
}

// Operation represents an operational transformation operation.
type Operation struct {
	Type      string `json:"type"`
	Position  int    `json:"position"`
	Content   string `json:"content,omitempty"`
	Length    int    `json:"length,omitempty"`
}

// Session represents a collaborative editing session.
type Session struct {
	ID           string              `json:"id"`
	PromptID     string              `json:"prompt_id"`
	Content      string              `json:"content"`
	Version      int                 `json:"version"`
	Cursors      map[string]*Cursor  `json:"cursors"`
	Changes      []*Change           `json:"changes"`
	Participants []string            `json:"participants"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// Manager manages collaborative editing sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	counter  atomic.Int64
}

// NewManager creates a new collaboration manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new collaborative editing session.
func (m *Manager) CreateSession(promptID, initialContent string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session := &Session{
		ID:       fmt.Sprintf("%d-%d", time.Now().UnixNano(), m.counter.Add(1)),
		PromptID: promptID,
		Content:  initialContent,
		Version:  0,
		Cursors:  make(map[string]*Cursor),
		Changes:  make([]*Change, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	m.sessions[session.ID] = session
	return session
}

// GetSession retrieves a session by ID.
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// ApplyChange applies a change to a session using OT.
func (m *Manager) ApplyChange(sessionID string, change *Change) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	
	// Apply operational transformation
	var newContent string
	switch change.Type {
	case "insert":
		if change.Position > len(session.Content) {
			change.Position = len(session.Content)
		}
		newContent = session.Content[:change.Position] + change.Content + session.Content[change.Position:]
	case "delete":
		if change.Position+change.Length > len(session.Content) {
			change.Length = len(session.Content) - change.Position
		}
		newContent = session.Content[:change.Position] + session.Content[change.Position+change.Length:]
	case "replace":
		if change.Position+change.Length > len(session.Content) {
			change.Length = len(session.Content) - change.Position
		}
		newContent = session.Content[:change.Position] + change.Content + session.Content[change.Position+change.Length:]
	default:
		return fmt.Errorf("unknown change type: %s", change.Type)
	}
	
	session.Content = newContent
	session.Version++
	change.Version = session.Version
	change.Timestamp = time.Now()
	session.Changes = append(session.Changes, change)
	session.UpdatedAt = time.Now()
	
	return nil
}

// UpdateCursor updates a user's cursor position.
func (m *Manager) UpdateCursor(sessionID string, cursor *Cursor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	
	cursor.Timestamp = time.Now()
	session.Cursors[cursor.UserID] = cursor
	
	// Add user to participants if not already there
	found := false
	for _, id := range session.Participants {
		if id == cursor.UserID {
			found = true
			break
		}
	}
	if !found {
		session.Participants = append(session.Participants, cursor.UserID)
	}
	
	return nil
}

// GetChanges returns changes since a given version.
func (m *Manager) GetChanges(sessionID string, sinceVersion int) []*Change {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil
	}
	
	var changes []*Change
	for _, change := range session.Changes {
		if change.Version > sinceVersion {
			changes = append(changes, change)
		}
	}
	return changes
}

// GetSessionContent returns the current content and version.
func (m *Manager) GetSessionContent(sessionID string) (string, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return "", 0, fmt.Errorf("session not found: %s", sessionID)
	}
	return session.Content, session.Version, nil
}

// CloseSession closes and removes a session.
func (m *Manager) CloseSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// GetActiveSessions returns all active sessions.
func (m *Manager) GetActiveSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}
