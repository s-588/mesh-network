package style

import "github.com/charmbracelet/lipgloss"

const (
	IconRREQ    = "\uf061"
	IconRREP    = "\uf060"
	IconDATA    = "\ued75"
	IconHELLO   = "\uf256"
	IconRERR    = "\uea87"
	IconForward = "\uf064"
	IconSend    = "\uf1d8"
	IconRecieve = "\uf265"
	IconBuffer  = "\uef96"
	IconDrop    = "\uf00d"
)

var (
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5BBA7D")).Bold(false)
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4A261")).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E76F51")).Bold(true)
	keyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3BFFA"))
	valStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
)
