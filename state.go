package main

import (
	"sync"
	"time"
)

type flowKind string

const (
	flowAdd    flowKind = "add"
	flowSearch flowKind = "search"
)

type flowState struct {
	Kind      flowKind
	Step      int
	Data      map[string]string
	UpdatedAt time.Time
}

type flowKey struct {
	ChatID int64
	UserID int64
}

type flowStore struct {
	mu    sync.Mutex
	flows map[flowKey]*flowState
}

func newFlowStore() *flowStore {
	return &flowStore{flows: map[flowKey]*flowState{}}
}

func (s *flowStore) get(chatID, userID int64) (*flowState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := flowKey{ChatID: chatID, UserID: userID}
	f, ok := s.flows[key]
	if !ok {
		return nil, false
	}
	return f, true
}

func (s *flowStore) set(chatID, userID int64, f *flowState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := flowKey{ChatID: chatID, UserID: userID}
	f.UpdatedAt = time.Now()
	s.flows[key] = f
}

func (s *flowStore) clear(chatID, userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := flowKey{ChatID: chatID, UserID: userID}
	delete(s.flows, key)
}

var flows = newFlowStore()
