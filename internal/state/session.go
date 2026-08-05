package state

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/AbdelilahOu/DBMcp/internal/driver"
)

type DBSessionState struct {
	Conn   *sql.DB
	Driver driver.Driver
}

var (
	current *DBSessionState
	mu      sync.RWMutex
)

func GetSession() *DBSessionState {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return nil
	}

	sessionCopy := *current
	return &sessionCopy
}

func SetSession(nextState *DBSessionState) *DBSessionState {
	if nextState == nil {
		return nil
	}

	mu.Lock()
	oldState := current
	stateCopy := *nextState
	current = &stateCopy
	mu.Unlock()

	if oldState != nil && oldState.Conn != nil && oldState.Conn != stateCopy.Conn {
		_ = oldState.Conn.Close()
	}

	return &stateCopy
}

func GetActiveSession() (*DBSessionState, error) {
	sessionState := GetSession()
	if sessionState == nil || sessionState.Conn == nil {
		return nil, fmt.Errorf("no active DB connection. Use switch_connection tool to connect to a database first")
	}

	return sessionState, nil
}

func CloseAllSessions() {
	mu.Lock()
	s := current
	current = nil
	mu.Unlock()

	if s != nil && s.Conn != nil {
		_ = s.Conn.Close()
	}
}
