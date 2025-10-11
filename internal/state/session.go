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
	return sessions[sessionID]
}

func CreateSession(sessionID string, globalClient *client.DBClient) *DBSessionState {
	mu.Lock()
	defer mu.Unlock()

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
	defer mu.Unlock()
	if s, ok := sessions[sessionID]; ok {
		s.Conn.Close()
		delete(sessions, sessionID)
	}
}

func OnDisconnect(sessionID string) {
	CloseSession(sessionID)
}
