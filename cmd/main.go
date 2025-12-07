package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hrutik5321/dbls/internal/app"
	"github.com/hrutik5321/dbls/internal/db/postgres"
)

func main() {
	// For now we always use Postgres. Later you can choose based on flags/env.
	pg := postgres.New()

	program := tea.NewProgram(app.New(pg))

	if _, err := program.Run(); err != nil {
		log.Fatalf("program failed: %v", err)
	}

	// Make sure DB is closed.
	if err := pg.Close(); err != nil {
		log.Printf("error closing DB: %v", err)
	}
}
