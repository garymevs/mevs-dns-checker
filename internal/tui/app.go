package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/garymevs/mevs-dns-checker/internal/dns"
	"github.com/garymevs/mevs-dns-checker/internal/email"
)

type mode int

const (
	modeInteractive mode = iota
	modeResults
)

type checkDoneMsg struct {
	summary dns.Summary
	report  *email.Report
	err     error
}

// Options configures the TUI session.
type Options struct {
	Domain        string
	Type          dns.RecordType
	DKIMSelectors []string
	Timeout       time.Duration
}

// Model is the Bubble Tea root model.
type Model struct {
	opts    Options
	client  *dns.Client
	checker *email.Checker
	mode    mode
	input   textinput.Model
	spinner spinner.Model
	loading bool
	domain  string // last checked domain (for recheck / tab switch)
	summary *dns.Summary
	report  *email.Report
	err     error
	width   int
	height  int
	typeIdx int
	status  string
}

// New creates the TUI model. If Domain is set, a check starts immediately.
func New(opts Options) Model {
	ti := textinput.New()
	ti.Placeholder = "example.com"
	ti.CharLimit = 253
	ti.Width = 40
	ti.Prompt = "› "
	if opts.Domain != "" {
		ti.SetValue(opts.Domain)
	} else {
		ti.Focus()
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAcc)

	client := dns.NewClient()
	if opts.Timeout > 0 {
		client.Timeout = opts.Timeout
	}

	m := Model{
		opts:    opts,
		client:  client,
		checker: email.NewChecker(client),
		mode:    modeInteractive,
		input:   ti,
		spinner: sp,
		domain:  opts.Domain,
		typeIdx: indexOfType(opts.Type),
	}
	if opts.Type == "" {
		m.typeIdx = 0
	}
	if opts.Domain != "" {
		m.loading = true
	}
	return m
}

func indexOfType(t dns.RecordType) int {
	for i, s := range dns.SupportedTypes {
		if s == t {
			return i
		}
	}
	return 0
}

func (m Model) Init() tea.Cmd {
	if m.opts.Domain != "" {
		return tea.Batch(m.spinner.Tick, m.runCheck(m.opts.Domain))
	}
	return textinput.Blink
}

func (m Model) currentType() dns.RecordType {
	if m.typeIdx < 0 || m.typeIdx >= len(dns.SupportedTypes) {
		return dns.TypeA
	}
	return dns.SupportedTypes[m.typeIdx]
}

func (m Model) runCheck(domain string) tea.Cmd {
	domain = strings.TrimSpace(domain)
	rtype := m.currentType()
	selectors := m.opts.DKIMSelectors
	client := m.client
	checker := m.checker

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if rtype == dns.TypeEMAIL {
			r := checker.Run(ctx, domain, selectors)
			return checkDoneMsg{
				summary: dns.Summary{Domain: domain, Type: rtype},
				report:  &r,
			}
		}

		sum := client.Lookup(ctx, domain, rtype)
		return checkDoneMsg{summary: sum}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case checkDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			m.mode = modeInteractive
			m.input.Focus()
			return m, textinput.Blink
		}
		sum := msg.summary
		m.summary = &sum
		m.report = msg.report
		m.domain = sum.Domain
		m.err = nil
		m.mode = modeResults
		m.status = ""
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.mode != modeInteractive {
				return m, tea.Quit
			}
		case "esc":
			if m.mode == modeResults {
				m.mode = modeInteractive
				m.input.Focus()
				return m, textinput.Blink
			}
			return m, tea.Quit
		}

		if m.loading {
			return m, nil
		}

		if m.mode == modeInteractive {
			return m.updateInteractive(msg)
		}
		return m.updateResults(msg)
	}

	if m.mode == modeInteractive {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateInteractive(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.typeIdx = (m.typeIdx + 1) % len(dns.SupportedTypes)
		return m, nil
	case "shift+tab":
		m.typeIdx--
		if m.typeIdx < 0 {
			m.typeIdx = len(dns.SupportedTypes) - 1
		}
		return m, nil
	case "enter":
		domain := strings.TrimSpace(m.input.Value())
		if domain == "" {
			m.status = "Enter a domain name"
			return m, nil
		}
		m.loading = true
		m.status = ""
		m.summary = nil
		m.report = nil
		m.domain = domain
		m.input.Blur()
		return m, tea.Batch(m.spinner.Tick, m.runCheck(domain))
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	domain := m.domain
	if domain == "" && m.summary != nil {
		domain = m.summary.Domain
	}

	switch msg.String() {
	case "r":
		if domain == "" {
			return m, nil
		}
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.runCheck(domain))
	case "tab", "right":
		m.typeIdx = (m.typeIdx + 1) % len(dns.SupportedTypes)
		if domain != "" {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.runCheck(domain))
		}
		return m, nil
	case "shift+tab", "left":
		m.typeIdx--
		if m.typeIdx < 0 {
			m.typeIdx = len(dns.SupportedTypes) - 1
		}
		if domain != "" {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.runCheck(domain))
		}
		return m, nil
	case "i", "esc":
		m.mode = modeInteractive
		m.input.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		m.width = 80
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("mevs-dns-checker"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Multi-resolver DNS + SPF / DKIM / DMARC"))
	b.WriteString("\n\n")

	if m.loading {
		label := "Querying public resolvers…"
		if m.currentType() == dns.TypeEMAIL {
			label = "Checking SPF / DKIM / DMARC…"
		}
		b.WriteString(m.spinner.View() + " " + label + "\n")
		return b.String()
	}

	switch m.mode {
	case modeInteractive:
		b.WriteString(m.viewForm())
	case modeResults:
		b.WriteString(m.viewResults())
	}

	return b.String()
}

func (m Model) viewForm() string {
	var b strings.Builder
	b.WriteString(headerStyle.Width(min(m.width-2, 72)).Render("Interactive lookup"))
	b.WriteString("\n")
	b.WriteString("Domain\n")
	b.WriteString(inputStyle.Render(m.input.View()))
	b.WriteString("\n\n")
	b.WriteString("Type  ")
	b.WriteString(m.renderTypePicker())
	b.WriteString("\n")
	b.WriteString(m.renderTypeDescription())
	b.WriteString("\n")
	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(failStyle.Render(m.status))
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("enter check · tab/shift+tab type · ctrl+c quit"))
	return b.String()
}

func (m Model) renderTypePicker() string {
	parts := make([]string, 0, len(dns.SupportedTypes))
	for i, t := range dns.SupportedTypes {
		label := string(t)
		if i == m.typeIdx {
			parts = append(parts, okStyle.Render("["+label+"]"))
		} else {
			parts = append(parts, muteStyle.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

func (m Model) renderTypeDescription() string {
	desc := m.currentType().Description()
	if desc == "" {
		return ""
	}
	return muteStyle.Render("      "+desc) + "\n"
}

func (m Model) viewResults() string {
	if m.summary == nil && m.report == nil {
		return failStyle.Render("No results")
	}

	rtype := m.currentType()
	domain := m.domain
	if m.summary != nil && domain == "" {
		domain = m.summary.Domain
	}

	var b strings.Builder
	b.WriteString(headerStyle.Width(min(m.width-2, 90)).Render(fmt.Sprintf("%s  ·  %s", domain, rtype)))
	b.WriteString("\n")
	b.WriteString(m.renderTypePicker())
	b.WriteString("\n")
	b.WriteString(m.renderTypeDescription())
	b.WriteString("\n")

	if rtype == dns.TypeEMAIL {
		b.WriteString(m.viewEmailResults())
	} else {
		b.WriteString(m.viewDNSResults())
	}

	b.WriteString(helpStyle.Render("r recheck · tab type · i/esc edit · q quit"))
	return b.String()
}

func (m Model) viewDNSResults() string {
	if m.summary == nil {
		return failStyle.Render("No DNS results") + "\n"
	}
	sum := m.summary
	var b strings.Builder

	agree := warnStyle.Render("differences across resolvers")
	if sum.IsAgreement() {
		agree = okStyle.Render("all resolvers agree")
	}
	b.WriteString(agree)
	b.WriteString("\n\n")

	rowFmt := "%-18s %-8s %-8s %s\n"
	b.WriteString(muteStyle.Render(fmt.Sprintf(rowFmt, "RESOLVER", "REGION", "RTT", "ANSWER")))
	for _, r := range sum.Results {
		name := r.Resolver.Name
		region := r.Resolver.Region
		rtt := formatRTT(r.RTT)
		var answer string
		style := lipgloss.NewStyle()
		if r.HasError() {
			answer = "ERR: " + r.Err.Error()
			style = failStyle
		} else if len(r.Values) == 0 {
			answer = "(empty)"
			style = muteStyle
		} else {
			answer = strings.Join(r.Values, ", ")
			if len(answer) > 60 {
				answer = answer[:57] + "..."
			}
		}
		line := fmt.Sprintf(rowFmt, name, region, rtt, answer)
		b.WriteString(style.Render(strings.TrimRight(line, "\n")))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Consensus groups"))
	b.WriteString("\n")
	for _, g := range sum.Consensus() {
		label := strings.Join(g.Values, ", ")
		st := okStyle
		if g.Error != "" {
			label = "ERR: " + g.Error
			st = failStyle
		} else if label == "" {
			label = "(empty)"
			st = muteStyle
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			st.Render(fmt.Sprintf("×%d", g.Count)),
			label,
		))
		b.WriteString(muteStyle.Render("    "+strings.Join(g.Resolvers, ", ")))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewEmailResults() string {
	if m.report == nil {
		return failStyle.Render("No email auth results") + "\n"
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("SPF / DKIM / DMARC"))
	b.WriteString("\n\n")
	b.WriteString(renderEmailCheck(m.report.SPF))
	b.WriteString(renderEmailCheck(m.report.DMARC))
	for _, d := range m.report.DKIM {
		b.WriteString(renderEmailCheck(d))
	}
	b.WriteString("\n")
	return b.String()
}

func renderEmailCheck(c email.CheckResult) string {
	var st lipgloss.Style
	switch c.Status {
	case email.StatusPass:
		st = okStyle
	case email.StatusWarn:
		st = warnStyle
	case email.StatusMissing:
		st = muteStyle
	default:
		st = failStyle
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %s %s — %s\n",
		st.Render(strings.ToUpper(string(c.Status))),
		c.Name,
		c.Summary,
	))
	for _, d := range c.Details {
		if len(d) > 100 {
			d = d[:97] + "..."
		}
		b.WriteString(muteStyle.Render("    · "+d))
		b.WriteString("\n")
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Run starts the Bubble Tea program.
func Run(opts Options) error {
	m := New(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
