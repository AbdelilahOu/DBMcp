package state

import (
	"database/sql"
	"sync"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/google/uuid"
)

type DBSessionState struct {
	Conn          *sql.DB
	CurrentSchema string
}

var (
	sessions = make(map[string]*DBSessionState)
	mu       sync.RWMutex
)

func GetOrCreateSession(sessionID string, globalClient *client.DBClient) *DBSessionState {
	mu.RLock()
	if s, ok := sessions[sessionID]; ok {
		mu.RUnlock()
		return s
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	// If no global client provided, create empty session that can be populated later
	if globalClient == nil {
		id := sessionID
		if id == "" {
			id = uuid.New().String()
		}
		s := &DBSessionState{
			Conn:          nil,
			CurrentSchema: "public",
		}
		sessions[id] = s
		return s
	}

	conn := globalClient.DB
	if err := conn.Ping(); err != nil {
		return nil
	}

	id := sessionID
	if id == "" {
		id = uuid.New().String()
	}
	s := &DBSessionState{
		Conn:          conn,
		CurrentSchema: "public",
	}
	sessions[id] = s
	return s
}

// GetSession returns an existing session without creating one
func GetSession(sessionID string) *DBSessionState {
	mu.RLock()
	defer mu.RUnlock()
	return sessions[sessionID]
}

func CloseSession(sessionID string) {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := sessions[sessionID]; ok {
		s.Conn.Close()
		delete(sessions, sessionID)
	}
}

func OnDisconnect(sessionID string) {
	CloseSession(sessionID)
}
