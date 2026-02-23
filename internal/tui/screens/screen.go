package screens

import tea "github.com/charmbracelet/bubbletea"

// Screen is the interface all screens implement.
type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (Screen, tea.Cmd)
	View() string
	SetSize(width, height int)
}
