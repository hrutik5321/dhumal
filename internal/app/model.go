package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hrutik5321/dbls/internal/db"
	"github.com/hrutik5321/dbls/internal/ui/table"
)

// ----- Modes -----

type mode int

const (
	modeForm mode = iota
	modeTables
	modeRows
)

// ----- Messages from async DB commands -----

type dbResultMsg struct {
	err error
}

type deleteResultMsg struct {
	affected int64
	err      error
}

type tablesResultMsg struct {
	tables []string
	err    error
}

type rowsResultMsg struct {
	page db.RowPage
	err  error
}

// ----- Model -----

type Model struct {
	dbClient db.DB

	// form inputs
	hostInput textinput.Model
	portInput textinput.Model
	userInput textinput.Model
	passInput textinput.Model
	dbInput   textinput.Model

	focusIndex int

	// state
	mode          mode
	status        string
	loading       bool
	tableNames    []string
	tableCursor   int
	selectedTable string

	columns []string
	rows    [][]string

	// pagination
	pageSize  int
	offset    int
	totalRows int

	// filtering
	filter        string
	filterInput   textinput.Model
	editingFilter bool

	// delete
	editingDelete bool

	// terminal / scroll
	width       int
	horizOffset int
}

// ----- Initial model -----

func initialModel(dbClient db.DB) Model {
	host := textinput.New()
	host.Placeholder = "localhost"
	host.Prompt = "Host: "

	port := textinput.New()
	port.Placeholder = "5432"
	port.Prompt = "Port: "

	user := textinput.New()
	user.Placeholder = "postgres"
	user.Prompt = "User: "

	pass := textinput.New()
	pass.Placeholder = "password"
	pass.Prompt = "Password: "
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '•'

	dbInput := textinput.New()
	dbInput.Placeholder = "database name"
	dbInput.Prompt = "Database: "

	filterInput := textinput.New()
	filterInput.Placeholder = "id > 10 AND status = 'active'"
	filterInput.Prompt = "WHERE "

	m := Model{
		dbClient:   dbClient,
		hostInput:  host,
		portInput:  port,
		userInput:  user,
		passInput:  pass,
		dbInput:    dbInput,
		focusIndex: 0,
		mode:       modeForm,
		status:     "Fill details and press Enter to connect.",
		pageSize:   10,
		offset:     0,
		totalRows:  0,

		filter:        "",
		filterInput:   filterInput,
		editingFilter: false,
	}

	m.hostInput.Focus()
	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// ----- Commands (async DB operations) -----

func connectCmd(client db.DB, cfg db.ConnConfig) tea.Cmd {
	return func() tea.Msg {
		err := client.Connect(context.Background(), cfg)
		return dbResultMsg{err: err}
	}
}

func deleteRowsCmd(client db.DB, tableName string, where string) tea.Cmd {
	return func() tea.Msg {
		affected, err := client.DeleteRows(context.Background(), tableName, where)
		return deleteResultMsg{affected: affected, err: err}
	}
}

func listTablesCmd(client db.DB) tea.Cmd {
	return func() tea.Msg {
		tables, err := client.ListTables(context.Background())
		return tablesResultMsg{tables: tables, err: err}
	}
}

func fetchRowsCmd(client db.DB, tableName string, opts db.QueryOptions) tea.Cmd {
	return func() tea.Msg {
		page, err := client.FetchRows(context.Background(), tableName, opts)
		return rowsResultMsg{page: page, err: err}
	}
}

// ----- Update -----

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// connection result
	case dbResultMsg:
		m.loading = false
		if msg.err != nil {
			m.status = "Connection failed: " + msg.err.Error()
			m.mode = modeForm
			return m, nil
		}

		m.status = "Connected! Fetching tables..."
		m.mode = modeTables
		m.loading = true
		return m, listTablesCmd(m.dbClient)

	// tables result
	case tablesResultMsg:
		m.loading = false
		if msg.err != nil {
			m.status = "Failed to fetch tables: " + msg.err.Error()
			m.mode = modeForm
			return m, nil
		}
		m.tableNames = msg.tables
		m.tableCursor = 0
		if len(msg.tables) == 0 {
			m.status = "Connected but no tables found in public schema."
		}
		return m, nil

	case deleteResultMsg:
		m.loading = false
		if msg.err != nil {
			m.status = "Delete failed: " + msg.err.Error()
			// stay in rows mode so user can adjust WHERE or try again
			m.mode = modeRows
			return m, nil
		}

		m.status = fmt.Sprintf("Deleted %d row(s). Reloading page...", msg.affected)
		// reload current page with same filter & offset (offset may adjust logically via rowsResultMsg)
		m.loading = true
		return m, fetchRowsCmd(
			m.dbClient,
			m.selectedTable,
			db.QueryOptions{
				Limit:  m.pageSize,
				Offset: m.offset,
				Filter: m.filter,
			},
		)

	// rows result (with pagination info)
	case rowsResultMsg:
		m.loading = false
		if msg.err != nil {
			m.status = "Failed to fetch rows: " + msg.err.Error()
			m.mode = modeTables
			return m, nil
		}
		m.columns = msg.page.Columns
		m.rows = msg.page.Rows
		m.totalRows = msg.page.TotalRows
		m.offset = msg.page.Offset
		m.status = fmt.Sprintf(
			"Showing rows (page size %d). Press 'b' to go back, 'n'/'p' for next/prev page, '/' to filter.",
			m.pageSize,
		)
		m.mode = modeRows
		return m, nil

	// window size
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// ----- Key handling dispatcher -----

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeForm:
		return m.updateFormKey(msg)
	case modeTables:
		return m.updateTablesKey(msg)
	case modeRows:
		return m.updateRowsKey(msg)
	default:
		return m, nil
	}
}

// --- form mode ---

func (m Model) updateFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "tab", "down":
		m.focusIndex++
		if m.focusIndex > 4 {
			m.focusIndex = 4
		}
	case "shift+tab", "up":
		m.focusIndex--
		if m.focusIndex < 0 {
			m.focusIndex = 0
		}
	case "enter":
		// if last field -> connect
		if m.focusIndex == 4 {
			m.loading = true
			m.status = "Connecting to DB..."
			return m, connectCmd(
				m.dbClient,
				db.ConnConfig{
					Host:     m.hostInput.Value(),
					Port:     m.portInput.Value(),
					User:     m.userInput.Value(),
					Password: m.passInput.Value(),
					Database: m.dbInput.Value(),
				},
			)
		}
		// otherwise move focus
		m.focusIndex++
		if m.focusIndex > 4 {
			m.focusIndex = 4
		}
	}

	// manage focus + inputs only in form mode
	cmds := m.updateFocus()
	switch m.focusIndex {
	case 0:
		var cmd tea.Cmd
		m.hostInput, cmd = m.hostInput.Update(msg)
		cmds = append(cmds, cmd)
	case 1:
		var cmd tea.Cmd
		m.portInput, cmd = m.portInput.Update(msg)
		cmds = append(cmds, cmd)
	case 2:
		var cmd tea.Cmd
		m.userInput, cmd = m.userInput.Update(msg)
		cmds = append(cmds, cmd)
	case 3:
		var cmd tea.Cmd
		m.passInput, cmd = m.passInput.Update(msg)
		cmds = append(cmds, cmd)
	case 4:
		var cmd tea.Cmd
		m.dbInput, cmd = m.dbInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// --- tables mode ---

func (m Model) updateTablesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "up":
		if m.tableCursor > 0 {
			m.tableCursor--
		}
	case "down":
		if m.tableCursor < len(m.tableNames)-1 {
			m.tableCursor++
		}
	case "enter":
		if len(m.tableNames) == 0 {
			return m, nil
		}
		m.selectedTable = m.tableNames[m.tableCursor]
		m.loading = true
		m.offset = 0
		m.horizOffset = 0
		m.filter = ""
		m.status = "Fetching rows from " + m.selectedTable + "..."
		return m, fetchRowsCmd(
			m.dbClient,
			m.selectedTable,
			db.QueryOptions{
				Limit:  m.pageSize,
				Offset: m.offset,
				Filter: m.filter,
			},
		)
	}
	return m, nil
}

// --- rows mode ---

func (m Model) updateRowsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// editing delete WHERE clause
	if m.editingDelete {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.editingDelete = false
			m.status = "Delete cancelled. Press 'd' to delete again."
			return m, nil

		case "enter":
			where := strings.TrimSpace(m.filterInput.Value())
			if where == "" {
				m.status = "WHERE clause cannot be empty for DELETE."
				return m, nil
			}
			m.editingDelete = false
			m.loading = true
			m.status = "Deleting rows..."
			return m, deleteRowsCmd(m.dbClient, m.selectedTable, where)
		}

		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	// editing filter
	if m.editingFilter {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.editingFilter = false
			m.filter = ""
			m.offset = 0
			m.loading = true
			m.status = "Filter cancelled. Press '/' to filter again."
			return m, fetchRowsCmd(
				m.dbClient,
				m.selectedTable,
				db.QueryOptions{
					Limit:  m.pageSize,
					Offset: m.offset,
					Filter: m.filter,
				},
			)
		case "enter":
			m.filter = m.filterInput.Value()
			m.editingFilter = false
			m.offset = 0
			m.loading = true
			m.status = "Applying filter..."
			return m, fetchRowsCmd(
				m.dbClient,
				m.selectedTable,
				db.QueryOptions{
					Limit:  m.pageSize,
					Offset: m.offset,
					Filter: m.filter,
				},
			)
		}

		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	// normal rows controls
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	// remove filters
	case "r":
		m.filter = ""
		m.offset = 0
		m.loading = true
		m.status = "Fetching rows from " + m.selectedTable + "..."
		return m, fetchRowsCmd(
			m.dbClient,
			m.selectedTable,
			db.QueryOptions{
				Limit:  m.pageSize,
				Offset: m.offset,
				Filter: m.filter,
			},
		)
	case "d":
		m.editingDelete = true
		m.editingFilter = false
		// m.filterInput.Prompt = "DELETE WHERE "
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		m.status = "Enter SQL WHERE clause for DELETE (without 'WHERE'). Enter to delete, Esc to cancel."
		return m, nil

	case "b":
		m.mode = modeTables
		m.status = "Use ↑/↓ and Enter to select another table."

	case "/":
		m.editingFilter = true
		m.editingDelete = false
		// m.filterInput.Prompt = "FILTER WHERE "
		m.filterInput.Placeholder = "Add Your Filter Here"
		m.filterInput.SetValue(m.filter)
		m.filterInput.Focus()
		m.status = "Enter SQL WHERE clause (without 'WHERE'). Enter to apply, Esc to cancel."
		return m, nil

	// pagination
	case "n":
		if m.totalRows == 0 {
			return m, nil
		}
		nextOffset := m.offset + m.pageSize
		if nextOffset >= m.totalRows {
			m.status = "Already at last page."
			return m, nil
		}
		m.loading = true
		m.status = "Loading next page..."
		return m, fetchRowsCmd(
			m.dbClient,
			m.selectedTable,
			db.QueryOptions{
				Limit:  m.pageSize,
				Offset: nextOffset,
				Filter: m.filter,
			},
		)

	case "p":
		if m.totalRows == 0 {
			return m, nil
		}
		prevOffset := m.offset - m.pageSize
		if prevOffset < 0 {
			prevOffset = 0
		}
		if prevOffset == m.offset {
			m.status = "Already at first page."
			return m, nil
		}
		m.loading = true
		m.status = "Loading previous page..."
		return m, fetchRowsCmd(
			m.dbClient,
			m.selectedTable,
			db.QueryOptions{
				Limit:  m.pageSize,
				Offset: prevOffset,
				Filter: m.filter,
			},
		)

	// fast horizontal scroll
	case "left", "h":
		m.horizOffset -= 4
		if m.horizOffset < 0 {
			m.horizOffset = 0
		}
	case "right", "l":
		m.horizOffset += 4
	case "shift+left":
		m.horizOffset -= 16
		if m.horizOffset < 0 {
			m.horizOffset = 0
		}
	case "shift+right":
		m.horizOffset += 16
	}

	return m, nil
}

// ----- Focus handling for form -----

func (m *Model) updateFocus() []tea.Cmd {
	var cmds []tea.Cmd

	m.hostInput.Blur()
	m.portInput.Blur()
	m.userInput.Blur()
	m.passInput.Blur()
	m.dbInput.Blur()

	switch m.focusIndex {
	case 0:
		cmds = append(cmds, m.hostInput.Focus())
	case 1:
		cmds = append(cmds, m.portInput.Focus())
	case 2:
		cmds = append(cmds, m.userInput.Focus())
	case 3:
		cmds = append(cmds, m.passInput.Focus())
	case 4:
		cmds = append(cmds, m.dbInput.Focus())
	}

	return cmds
}

// ----- Views -----

func (m Model) View() string {
	switch m.mode {
	case modeForm:
		return m.viewForm()
	case modeTables:
		return m.viewTables()
	case modeRows:
		return m.viewRows()
	default:
		return "Unknown state"
	}
}

func (m Model) viewForm() string {
	loading := ""
	if m.loading {
		loading = "\n\n[Working...]"
	}

	return fmt.Sprintf(
		"Enter Postgres Credentials:\n\n%s\n%s\n%s\n%s\n%s\n\n%s%s\n\n(ctrl+c/esc to quit)\n",
		m.hostInput.View(),
		m.portInput.View(),
		m.userInput.View(),
		m.passInput.View(),
		m.dbInput.View(),
		m.status,
		loading,
	)
}

func (m Model) viewTables() string {
	s := "Connected.\n\nTables in public schema:\n\n"

	if len(m.tableNames) == 0 && !m.loading {
		s += "  (no tables found)\n"
	}

	for i, t := range m.tableNames {
		cursor := "  "
		if i == m.tableCursor {
			cursor = "> "
		}
		s += fmt.Sprintf("%s%s\n", cursor, t)
	}

	if m.loading {
		s += "\nLoading...\n"
	}

	s += "\n" + m.status + "\n"
	s += "\nUse ↑/↓ and Enter. Press q or ctrl+c to quit.\n"

	return s
}

func (m Model) viewRows() string {
	s := fmt.Sprintf("Rows from table: %s\n\n", m.selectedTable)

	if m.filter != "" {
		s += fmt.Sprintf("Active filter: WHERE %s\n\n", m.filter)
	}

	if len(m.columns) == 0 {
		s += "(No rows or columns found)\n"
	} else {
		s += table.Render(m.columns, m.rows)
	}

	if m.filter != "" {
		s += "\nPress 'r' to refresh the table (clear filter)\n"
	}

	// Pagination info
	if m.totalRows > 0 {
		start := m.offset + 1
		end := m.offset + len(m.rows)
		if end > m.totalRows {
			end = m.totalRows
		}
		totalPages := (m.totalRows + m.pageSize - 1) / m.pageSize
		currentPage := (m.offset / m.pageSize) + 1

		s += fmt.Sprintf(
			"\nRows %d–%d of %d (Page %d/%d, page size %d)\n",
			start, end, m.totalRows, currentPage, totalPages, m.pageSize,
		)
	} else {
		s += "\n(No rows)\n"
	}

	if m.editingFilter {
		// 	input := m.filterInput.View()
		// label := " DELETE WHERE "
		// s += "\nFilter: " + m.filterInput.View() + "\n"
		input := m.filterInput.View()
		label := "Filter"

		top := "┌" + strings.Repeat("─", len(input)+2) + "┐"
		middle := "│ " + input + " "
		bottom := "└" + strings.Repeat("─", len(input)+2) + "┘"

		s += "\n" + label + "\n" + top + "\n" + middle + "\n" + bottom + "\n"
	}

	if m.editingDelete {
		input := m.filterInput.View()
		label := " DELETE "

		top := "┌" + strings.Repeat("─", len(input)+2) + "┐"
		middle := "│ " + input + " "
		bottom := "└" + strings.Repeat("─", len(input)+2) + "┘"

		s += "\n" + label + "\n" + top + "\n" + middle + "\n" + bottom + "\n"
	}

	s += "\n" + m.status + "\n"
	s += "\nPress 'b' to go back to tables, 'q' or ctrl+c to quit. Use n/p for next/prev page, '/' to filter, ←/→ or h/l to scroll horizontally.\n"

	// apply horizontal scroll based on terminal width and offset
	return table.ApplyHorizontalScroll(s, m.horizOffset, m.width)
}
