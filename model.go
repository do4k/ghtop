package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── styles ────────────────────────────────────────────────────────────────────

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleDivider  = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	styleSuccess  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleFailure  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleRunning  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	stylePending  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleMuted    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleRepo     = lipgloss.NewStyle().Bold(true)
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleHelp     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stylePrompt   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleInputErr = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)

// ── messages ──────────────────────────────────────────────────────────────────

type runResultMsg struct {
	key    string
	result RunStatus
}

type autoRefreshMsg struct{}

// ── model ─────────────────────────────────────────────────────────────────────

type appState int

const (
	stateList   appState = iota
	stateAdding          // text input overlay
)

type model struct {
	state    appState
	pins     []Pin
	statuses []RunStatus
	loading  map[string]bool
	cursor   int
	input    textinput.Model
	inputErr string
	spinner  spinner.Model
	width    int
	height   int
}

func newModel(pins []Pin) model {
	ti := textinput.New()
	ti.Placeholder = "owner/repo/actions/runs/ID  or  full URL"
	ti.CharLimit = 256
	ti.Width = 64

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleRunning

	statuses := make([]RunStatus, len(pins))
	for i, p := range pins {
		statuses[i] = RunStatus{Pin: p}
	}
	loading := make(map[string]bool, len(pins))
	for _, p := range pins {
		loading[p.Key()] = true
	}

	return model{
		state:    stateList,
		pins:     pins,
		statuses: statuses,
		loading:  loading,
		input:    ti,
		spinner:  sp,
		width:    80,
		height:   24,
	}
}

// ── commands ──────────────────────────────────────────────────────────────────

func fetchCmd(pin Pin) tea.Cmd {
	return func() tea.Msg {
		return runResultMsg{key: pin.Key(), result: fetchRunStatus(pin)}
	}
}

func autoRefreshCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg { return autoRefreshMsg{} })
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick, autoRefreshCmd()}
	for _, p := range m.pins {
		cmds = append(cmds, fetchCmd(p))
	}
	return tea.Batch(cmds...)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case runResultMsg:
		for i, p := range m.pins {
			if p.Key() == msg.key {
				m.statuses[i] = msg.result
				delete(m.loading, msg.key)
				break
			}
		}

	case autoRefreshMsg:
		for _, p := range m.pins {
			if !m.loading[p.Key()] {
				m.loading[p.Key()] = true
				cmds = append(cmds, fetchCmd(p))
			}
		}
		cmds = append(cmds, autoRefreshCmd())

	case tea.KeyMsg:
		switch m.state {
		case stateAdding:
			switch msg.Type {
			case tea.KeyEnter:
				pin, err := parseRunURL(m.input.Value())
				if err != nil {
					m.inputErr = err.Error()
				} else {
					dup := false
					for _, p := range m.pins {
						if p.Key() == pin.Key() {
							dup = true
							break
						}
					}
					if dup {
						m.inputErr = "already pinned"
					} else {
						m.pins = append(m.pins, pin)
						m.statuses = append(m.statuses, RunStatus{Pin: pin})
						m.loading[pin.Key()] = true
						m.cursor = len(m.pins) - 1
						_ = savePins(m.pins)
						m.state = stateList
						m.input.Blur()
						m.input.Reset()
						m.inputErr = ""
						cmds = append(cmds, fetchCmd(pin))
					}
				}
			case tea.KeyEscape:
				m.state = stateList
				m.input.Blur()
				m.input.Reset()
				m.inputErr = ""
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}

		case stateList:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "a":
				m.state = stateAdding
				cmd := m.input.Focus()
				cmds = append(cmds, cmd)
			case "d":
				if len(m.pins) > 0 {
					delete(m.loading, m.pins[m.cursor].Key())
					m.pins = sliceDelete(m.pins, m.cursor)
					m.statuses = sliceDeleteStatus(m.statuses, m.cursor)
					if m.cursor >= len(m.pins) && m.cursor > 0 {
						m.cursor--
					}
					_ = savePins(m.pins)
				}
			case "r":
				for _, p := range m.pins {
					if !m.loading[p.Key()] {
						m.loading[p.Key()] = true
						cmds = append(cmds, fetchCmd(p))
					}
				}
			case "R":
				if len(m.pins) > 0 {
					p := m.pins[m.cursor]
					m.loading[p.Key()] = true
					cmds = append(cmds, fetchCmd(p))
				}
			case "o", "enter":
				if len(m.pins) > 0 {
					openRunInBrowser(m.pins[m.cursor])
				}
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.pins)-1 {
					m.cursor++
				}
			}
		}
	}

	// Forward non-key messages to the textinput (cursor blink etc.)
	if m.state == stateAdding {
		if _, isKey := msg.(tea.KeyMsg); !isKey {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func sliceDelete(s []Pin, i int) []Pin {
	out := make([]Pin, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}

func sliceDeleteStatus(s []RunStatus, i int) []RunStatus {
	out := make([]RunStatus, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	var sb strings.Builder
	w := m.width
	if w < 40 {
		w = 80
	}

	divider := styleDivider.Render(strings.Repeat("─", w))

	sb.WriteString(styleTitle.Render("ghtop") + styleMuted.Render("  GitHub Actions Monitor") + "\n")
	sb.WriteString(divider + "\n")

	if m.state == stateAdding {
		sb.WriteString("\n")
		sb.WriteString(stylePrompt.Render("  Pin a run") + "\n\n")
		sb.WriteString("  " + m.input.View() + "\n")
		if m.inputErr != "" {
			sb.WriteString("\n  " + styleInputErr.Render("✗  "+m.inputErr) + "\n")
		}
		if ghe := gheHostname(); ghe != "" {
			sb.WriteString("\n  " + styleMuted.Render("default host: "+ghe) + "\n")
		}
		sb.WriteString("\n")
		sb.WriteString(styleHelp.Render("  enter to confirm  ·  esc to cancel") + "\n")
		return sb.String()
	}

	sb.WriteString("\n")
	if len(m.pins) == 0 {
		sb.WriteString(styleMuted.Render("  No pinned runs.  Press 'a' to add one.") + "\n")
	} else {
		for i, s := range m.statuses {
			sb.WriteString(m.renderRun(s, i == m.cursor))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(divider + "\n")
	sb.WriteString(styleHelp.Render("  a add  d delete  r refresh all  R refresh  o open  j/k ↑/↓  q quit"))

	return sb.String()
}

func (m model) renderRun(s RunStatus, selected bool) string {
	var sb strings.Builder

	cursor := "   "
	if selected {
		cursor = styleCursor.Render(" ► ")
	}

	isLoading := m.loading[s.Pin.Key()]
	iconStr := m.runIconStr(s, isLoading)
	// pad lines 2/3 to align under the repo name:
	// cursor(3) + icon(2) + space(1) = 6
	pad := strings.Repeat(" ", 3+2+1)

	// Line 1: [cursor][icon] [host/]owner/repo  #run
	runRef := "#" + s.Pin.RunID
	if s.Number > 0 {
		runRef = fmt.Sprintf("#%d", s.Number)
	}
	repoLabel := s.Pin.RepoSlug()
	hostLabel := ""
	if s.Pin.Hostname != "" && s.Pin.Hostname != "github.com" {
		hostLabel = styleMuted.Render(s.Pin.Hostname + "/")
	}
	sb.WriteString(cursor + iconStr + " " +
		hostLabel + styleRepo.Render(repoLabel) +
		"  " + styleMuted.Render(runRef) + "\n")

	// Line 2: workflow title  OR  error  OR  initial-fetch spinner
	if s.FetchError != "" {
		sb.WriteString(pad + styleError.Render(truncate(s.FetchError, 74)) + "\n")
	} else if isLoading && s.Status == "" {
		sb.WriteString(pad + m.spinner.View() + styleMuted.Render(" fetching…") + "\n")
	} else {
		title := s.DisplayTitle
		if title == "" {
			title = s.WorkflowName
		}
		if title != "" {
			sb.WriteString(pad + title + "\n")
		}

		// Line 3: branch · 🏷 conclusion · age [· refresh spinner]
		dot := styleMuted.Render(" · ")
		var seg []string
		if s.HeadBranch != "" {
			seg = append(seg, styleMuted.Render(s.HeadBranch))
		}
		if s.Status != "" || s.Conclusion != "" {
			seg = append(seg, conclusionText(s))
		}
		if !s.UpdatedAt.IsZero() {
			seg = append(seg, styleMuted.Render(timeAgo(s.UpdatedAt)))
		}
		if len(seg) > 0 || isLoading {
			line := strings.Join(seg, dot)
			if isLoading {
				line += "  " + m.spinner.View()
			}
			sb.WriteString(pad + line + "\n")
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// runIconStr returns a 2-column-wide styled icon for line 1.
func (m model) runIconStr(s RunStatus, isLoading bool) string {
	pad1 := func(styled string) string { return styled + " " } // widen 1-col symbol to 2
	if isLoading && s.Status == "" {
		return pad1(stylePending.Render("○"))
	}
	switch s.Status {
	case "completed":
		switch s.Conclusion {
		case "success":
			return "✅"
		case "failure":
			return "❌"
		case "timed_out":
			return pad1(styleFailure.Render("⏱"))
		case "cancelled":
			return pad1(styleMuted.Render("⊘"))
		case "skipped":
			return pad1(styleMuted.Render("◌"))
		case "action_required":
			return "⚠️"
		default:
			return pad1(styleMuted.Render("●"))
		}
	case "in_progress":
		return m.spinner.View() + " " // spinner is 1-col
	case "queued", "waiting", "pending":
		return "⏳"
	default:
		return pad1(styleMuted.Render("○"))
	}
}

// conclusionText returns a coloured, emoji-prefixed status string for line 3.
func conclusionText(s RunStatus) string {
	if s.Status == "completed" {
		switch s.Conclusion {
		case "success":
			return styleSuccess.Render("✓ success")
		case "failure":
			return styleFailure.Render("✗ failure")
		case "timed_out":
			return styleFailure.Render("⏱ timed out")
		case "cancelled":
			return styleMuted.Render("⊘ cancelled")
		case "skipped":
			return styleMuted.Render("◌ skipped")
		case "action_required":
			return styleRunning.Render("⚠ action required")
		default:
			return styleMuted.Render(s.Conclusion)
		}
	}
	switch s.Status {
	case "in_progress":
		return styleRunning.Render("running")
	case "queued":
		return stylePending.Render("⏳ queued")
	case "waiting":
		return stylePending.Render("⏳ waiting")
	default:
		return styleMuted.Render(strings.ReplaceAll(s.Status, "_", " "))
	}
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
