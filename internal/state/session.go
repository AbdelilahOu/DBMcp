package state

import (
	"database/sql"
	"sync"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/google/uuid"
)

// DBSessionState holds per-session DB state (conn, context like current schema)
type DBSessionState struct {
	Conn          *sql.DB
	CurrentSchema string // E.g., "public" for Postgres
	// Add Tx *sql.Tx for transactions later
}

// Global session map (thread-safe, like GitHub MCP's per-session repo context)
var (
	sessions = make(map[string]*DBSessionState)
	mu       sync.RWMutex
)

// GetOrCreateSession retrieves or creates session state (init conn from global client if needed)
func GetOrCreateSession(sessionID string, globalClient *client.DBClient) *DBSessionState {
	mu.RLock()
	if s, ok := sessions[sessionID]; ok {
		mu.RUnlock()
		return s
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	// Create new (lazy conn open per session for isolation)
	// In prod, could share global pool; here, per-session for safety
	// For now, reuse the global client's connection for simplicity
	conn := globalClient.DB
	if err := conn.Ping(); err != nil {
		// Return nil session on connection failure
		return nil
	}

	id := sessionID // Use provided ID
	if id == "" {
		id = uuid.New().String()
	}
	s := &DBSessionState{
		Conn:          conn,
		CurrentSchema: "public", // Default
	}
	sessions[id] = s
	return s
}

// CloseSession cleans up resources on disconnect/timeout
func CloseSession(sessionID string) {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := sessions[sessionID]; ok {
		s.Conn.Close()
		delete(sessions, sessionID)
	}
}

// Middleware-like: Optional server.OnDisconnect hook (add to server opts in main.go for prod)
func OnDisconnect(sessionID string) {
	CloseSession(sessionID)
}
