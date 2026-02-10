package main

import (
	"fmt"
	"sync"
)

type Todo struct {
	ID    string
	Title string
}

type TodoStore interface {
	List() ([]Todo, error)
	Add(title string) error
	Delete(id string) error
}

// MemoryStore stores todos in memory. Data is lost when the process exits.
type MemoryStore struct {
	mu     sync.Mutex
	todos  []Todo
	nextID int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) List() ([]Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Todo, len(s.todos))
	copy(out, s.todos)
	return out, nil
}

func (s *MemoryStore) Add(title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.todos = append(s.todos, Todo{
		ID:    fmt.Sprintf("%d", s.nextID),
		Title: title,
	})
	return nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.todos {
		if t.ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			return nil
		}
	}
	return nil
}
