package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresStore stores todos in a PostgreSQL database.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id    SERIAL PRIMARY KEY,
		title TEXT NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) List() ([]Todo, error) {
	rows, err := s.db.Query("SELECT id, title FROM todos ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func (s *PostgresStore) Add(title string) error {
	_, err := s.db.Exec("INSERT INTO todos (title) VALUES ($1)", title)
	return err
}

func (s *PostgresStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM todos WHERE id = $1", id)
	return err
}
