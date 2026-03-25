package state

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/AbdelilahOu/DBMcp/internal/client"
	"github.com/google/uuid"
)

type DBSessionState struct {
	Conn          *sql.DB
	CurrentSchema string
	DBType        string
}

var (
	sessions = make(map[string]*DBSessionState)
	mu       sync.RWMutex
)

func GetSession(sessionID string) *DBSessionState {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := sessions[sessionID]
	if !ok || s == nil {
		return nil
	}

	sessionCopy := *s
	return &sessionCopy
}

func CreateSession(sessionID string, globalClient *client.DBClient) *DBSessionState {
	id := sessionID
	if id == "" {
		id = uuid.New().String()
	}

	nextState := &DBSessionState{
		CurrentSchema: "public",
	}
	if globalClient == nil {
		return SetSession(id, nextState)
	}

	conn := globalClient.DB
	if conn == nil {
		return nil
	}
	if err := conn.Ping(); err != nil {
		return nil
	}

	nextState.Conn = conn
	return SetSession(id, nextState)
}

func SetSession(sessionID string, nextState *DBSessionState) *DBSessionState {
	if nextState == nil {
		return nil
	}

	id := sessionID
	if id == "" {
		id = uuid.New().String()
	}

	mu.Lock()
	oldState := sessions[id]

	stateCopy := *nextState
	sessions[id] = &stateCopy
	mu.Unlock()

	if oldState != nil && oldState.Conn != nil && oldState.Conn != stateCopy.Conn {
		_ = oldState.Conn.Close()
	}

	return &stateCopy
}

func GetActiveSession(sessionID string) (*DBSessionState, error) {
	if sessionID == "" {
		sessionID = "default"
	}

	sessionState := GetSession(sessionID)
	if sessionState == nil {
		return nil, fmt.Errorf("no active DB connection. Use switch_connection tool to connect to a database first")
	}

	if sessionState.Conn == nil {
		return nil, fmt.Errorf("no active DB connection. Use switch_connection tool to connect to a database first")
	}

	return sessionState, nil
}

func CloseSession(sessionID string) {
	mu.Lock()
	s, ok := sessions[sessionID]
	if ok {
		delete(sessions, sessionID)
	}
	mu.Unlock()

	if ok && s != nil && s.Conn != nil {
		_ = s.Conn.Close()
	}
}

func CloseAllSessions() {
	mu.Lock()
	currentSessions := sessions
	sessions = make(map[string]*DBSessionState)
	mu.Unlock()

	closed := make(map[*sql.DB]struct{})
	for _, s := range currentSessions {
		if s == nil || s.Conn == nil {
			continue
		}
		if _, exists := closed[s.Conn]; exists {
			continue
		}
		closed[s.Conn] = struct{}{}
		_ = s.Conn.Close()
	}
}

func OnDisconnect(sessionID string) {
	CloseSession(sessionID)
}
