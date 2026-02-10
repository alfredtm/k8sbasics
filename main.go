package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var tmpl *template.Template

type pageData struct {
	Todos   []Todo
	Backend string
}

func main() {
	// Parse templates
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	// Try templates relative to executable first, then working directory
	tmplPath := filepath.Join(exeDir, "templates", "index.html")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = filepath.Join("templates", "index.html")
	}
	var err error
	tmpl, err = template.ParseFiles(tmplPath)
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	// Choose storage backend
	var store TodoStore
	var backend string
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		s, err := NewPostgresStore(dbURL)
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		store = s
		backend = "PostgreSQL (persistent)"
		log.Println("Using PostgreSQL store")
	} else {
		store = NewMemoryStore()
		backend = "In-Memory (ephemeral)"
		log.Println("Using in-memory store")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		todos, err := store.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, pageData{Todos: todos, Backend: backend})
	})

	http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		title := r.FormValue("title")
		if title != "" {
			if err := store.Add(title); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		id := r.FormValue("id")
		if id != "" {
			if err := store.Delete(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	addr := ":8080"
	log.Printf("Listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
