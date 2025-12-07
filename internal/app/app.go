package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hrutik5321/dhumal/internal/db"
)

func New(dbClient db.DB) tea.Model {
	return initialModel(dbClient)
}

func NewProgram(dbClient db.DB) *tea.Program {
	return tea.NewProgram(New(dbClient))
}
