package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/garymevs/mevs-dns-checker/internal/dns"
	"github.com/garymevs/mevs-dns-checker/internal/email"
)

type mode int

const (
	modeInteractive mode = iota
	modeResults
	modeDKIM // edit DKIM selectors after a search
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
	opts          Options
	client        *dns.Client
	checker       *email.Checker
	mode          mode
	input         textinput.Model
	dkimInput     textinput.Model
	dkimSelectors []string
	spinner       spinner.Model
	viewport      viewport.Model
	loading       bool
	domain        string
	checkedType   dns.RecordType
	summary       *dns.Summary
	report        *email.Report
	err           error
	width         int
	height        int
	typeIdx       int
	status        string
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

	dkim := textinput.New()
	dkim.Placeholder = "google,selector1 (comma-separated; empty = common list)"
	dkim.CharLimit = 200
	dkim.Width = 50
	dkim.Prompt = "› "
	if len(opts.DKIMSelectors) > 0 {
		dkim.SetValue(strings.Join(opts.DKIMSelectors, ","))
	}

	client := dns.NewClient()
	if opts.Timeout > 0 {
		client.Timeout = opts.Timeout
	}

	m := Model{
		opts:          opts,
		client:        client,
		checker:       email.NewChecker(client),
		mode:          modeInteractive,
		input:         ti,
		dkimInput:     dkim,
		dkimSelectors: append([]string(nil), opts.DKIMSelectors...),
		spinner:       sp,
		viewport:      viewport.New(80, 20),
		domain:        opts.Domain,
		typeIdx:       indexOfType(opts.Type),
		width:         80,
		height:        24,
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

func parseSelectors(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (m *Model) selectEMAILType() {
	m.typeIdx = indexOfType(dns.TypeEMAIL)
}

func (m Model) runCheck(domain string) tea.Cmd {
	domain = strings.TrimSpace(domain)
	rtype := m.currentType()
	selectors := append([]string(nil), m.dkimSelectors...)
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

func (m *Model) syncViewport() {
	if m.mode != modeResults {
		return
	}
	header := m.resultsHeader()
	footer := m.resultsFooter()
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	titleHeight := 3 // title + subtitle + blank
	avail := m.height - titleHeight - headerHeight - footerHeight
	if avail < 3 {
		avail = 3
	}
	m.viewport.Width = max(1, m.width)
	m.viewport.Height = avail
	m.viewport.SetContent(m.resultsBody())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = min(40, max(10, msg.Width-8))
		m.dkimInput.Width = min(60, max(20, msg.Width-12))
		m.syncViewport()
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
		m.checkedType = sum.Type
		m.err = nil
		m.mode = modeResults
		m.status = ""
		m.syncViewport()
		m.viewport.GotoTop()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// Don't steal q while typing domain or DKIM selectors.
			if m.mode == modeResults {
				return m, tea.Quit
			}
		case "esc":
			switch m.mode {
			case modeDKIM:
				m.dkimInput.Blur()
				m.mode = modeResults
				m.syncViewport()
				return m, nil
			case modeResults:
				m.mode = modeInteractive
				m.input.Focus()
				return m, textinput.Blink
			default:
				return m, tea.Quit
			}
		}

		if m.loading {
			return m, nil
		}

		switch m.mode {
		case modeInteractive:
			return m.updateInteractive(msg)
		case modeDKIM:
			return m.updateDKIM(msg)
		default:
			return m.updateResults(msg)
		}
	}

	if m.mode == modeInteractive {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	if m.mode == modeDKIM {
		var cmd tea.Cmd
		m.dkimInput, cmd = m.dkimInput.Update(msg)
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
	case "enter", "r":
		if domain == "" {
			return m, nil
		}
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.runCheck(domain))
	case "s":
		if len(m.dkimSelectors) > 0 {
			m.dkimInput.SetValue(strings.Join(m.dkimSelectors, ","))
		} else {
			m.dkimInput.SetValue("")
		}
		m.dkimInput.CursorEnd()
		m.dkimInput.Focus()
		m.mode = modeDKIM
		return m, textinput.Blink
	case "tab":
		m.typeIdx = (m.typeIdx + 1) % len(dns.SupportedTypes)
		m.syncViewport()
		return m, nil
	case "shift+tab":
		m.typeIdx--
		if m.typeIdx < 0 {
			m.typeIdx = len(dns.SupportedTypes) - 1
		}
		m.syncViewport()
		return m, nil
	case "i":
		m.mode = modeInteractive
		m.input.Focus()
		return m, textinput.Blink
	case "left", "right":
		if msg.String() == "right" {
			m.typeIdx = (m.typeIdx + 1) % len(dns.SupportedTypes)
		} else {
			m.typeIdx--
			if m.typeIdx < 0 {
				m.typeIdx = len(dns.SupportedTypes) - 1
			}
		}
		m.syncViewport()
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) updateDKIM(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.dkimSelectors = parseSelectors(m.dkimInput.Value())
		m.dkimInput.Blur()
		m.selectEMAILType()
		domain := m.domain
		if domain == "" && m.summary != nil {
			domain = m.summary.Domain
		}
		if domain == "" {
			m.mode = modeInteractive
			m.input.Focus()
			m.status = "Enter a domain, then re-check EMAIL"
			return m, textinput.Blink
		}
		m.loading = true
		m.mode = modeResults
		return m, tea.Batch(m.spinner.Tick, m.runCheck(domain))
	}

	var cmd tea.Cmd
	m.dkimInput, cmd = m.dkimInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
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
	case modeDKIM:
		b.WriteString(m.viewDKIMForm())
	case modeResults:
		b.WriteString(m.resultsHeader())
		b.WriteString(m.viewport.View())
		b.WriteString("\n")
		b.WriteString(m.resultsFooter())
	}

	return b.String()
}

func (m Model) viewDKIMForm() string {
	domain := m.domain
	if domain == "" {
		domain = "(no domain yet)"
	}
	var b strings.Builder
	b.WriteString(headerStyle.Width(contentWidth(m.width)).Render("DKIM selectors"))
	b.WriteString("\n")
	b.WriteString(muteStyle.Render("Domain: "+domain))
	b.WriteString("\n\n")
	b.WriteString("Selectors (comma-separated)\n")
	b.WriteString(inputStyle.Render(m.dkimInput.View()))
	b.WriteString("\n\n")
	b.WriteString(muteStyle.Render("Leave empty to probe the common selector list."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter check EMAIL · esc cancel"))
	return b.String()
}

func (m Model) viewForm() string {
	var b strings.Builder
	b.WriteString(headerStyle.Width(contentWidth(m.width)).Render("Interactive lookup"))
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
	w := contentWidth(m.width)
	return muteStyle.Render(wrapLine("      "+desc, w)) + "\n"
}

func (m Model) resultsHeader() string {
	if m.summary == nil && m.report == nil {
		return failStyle.Render("No results") + "\n"
	}

	selected := m.currentType()
	shown := m.checkedType
	if shown == "" && m.summary != nil {
		shown = m.summary.Type
	}
	pending := selected != shown

	domain := m.domain
	if m.summary != nil && domain == "" {
		domain = m.summary.Domain
	}

	var b strings.Builder
	b.WriteString(headerStyle.Width(contentWidth(m.width)).Render(fmt.Sprintf("%s  ·  %s", domain, shown)))
	b.WriteString("\n")
	b.WriteString(m.renderTypePicker())
	b.WriteString("\n")
	b.WriteString(m.renderTypeDescription())
	if pending {
		b.WriteString(warnStyle.Render(fmt.Sprintf("      enter to check %s", selected)))
		b.WriteString("\n")
	}
	if shown == dns.TypeEMAIL || selected == dns.TypeEMAIL {
		sel := "common list"
		if len(m.dkimSelectors) > 0 {
			sel = strings.Join(m.dkimSelectors, ", ")
		}
		b.WriteString(muteStyle.Render(wrapLine("      DKIM: "+sel+"  (s to set)", contentWidth(m.width))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) resultsFooter() string {
	return helpStyle.Render("tab type · enter/r check · s dkim · ↑↓/pg scroll · i/esc edit · q quit")
}

func (m Model) resultsBody() string {
	shown := m.checkedType
	if shown == "" && m.summary != nil {
		shown = m.summary.Type
	}
	if shown == dns.TypeEMAIL {
		return m.viewEmailResults()
	}
	return m.viewDNSResults()
}

func (m Model) viewDNSResults() string {
	if m.summary == nil {
		return failStyle.Render("No DNS results") + "\n"
	}
	sum := m.summary
	w := contentWidth(m.width)
	var b strings.Builder

	agree := warnStyle.Render("differences across resolvers")
	if sum.IsAgreement() {
		agree = okStyle.Render("all resolvers agree")
	}
	b.WriteString(agree)
	b.WriteString("\n\n")

	metaW := 18 + 1 + 8 + 1 + 8 + 1 // name region rtt spaces
	valueW := w - metaW
	if valueW < 20 {
		valueW = 20
	}

	b.WriteString(muteStyle.Render(fmt.Sprintf("%-18s %-8s %-8s %s", "RESOLVER", "REGION", "RTT", "ANSWER")))
	b.WriteString("\n")

	for _, r := range sum.Results {
		name := r.Resolver.Name
		region := r.Resolver.Region
		rtt := formatRTT(r.RTT)
		prefix := fmt.Sprintf("%-18s %-8s %-8s ", name, region, rtt)
		cont := strings.Repeat(" ", len(prefix))

		var block string
		style := lipgloss.NewStyle()
		switch {
		case r.HasError():
			block = wrapLine(prefix+"ERR: "+r.Err.Error(), w)
			style = failStyle
		case len(r.Values) == 0:
			block = prefix + "(empty)"
			style = muteStyle
		case len(r.Values) == 1 && utf8Len(r.Values[0]) <= valueW:
			block = prefix + r.Values[0]
		default:
			block = formatValuesBlock(r.Values, prefix, cont, w)
		}
		b.WriteString(style.Render(block))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Consensus groups"))
	b.WriteString("\n")
	for _, g := range sum.Consensus() {
		count := okStyle.Render(fmt.Sprintf("×%d", g.Count))
		if g.Error != "" {
			b.WriteString(fmt.Sprintf("  %s  %s\n", count, failStyle.Render("ERR: "+g.Error)))
		} else if len(g.Values) == 0 {
			b.WriteString(fmt.Sprintf("  %s  %s\n", count, muteStyle.Render("(empty)")))
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", count))
			for _, v := range g.Values {
				b.WriteString(wrapLine("    · "+v, w))
				b.WriteString("\n")
			}
		}
		b.WriteString(muteStyle.Render(wrapLine("    "+strings.Join(g.Resolvers, ", "), w)))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewEmailResults() string {
	if m.report == nil {
		return failStyle.Render("No email auth results") + "\n"
	}
	w := contentWidth(m.width)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("SPF / DKIM / DMARC"))
	b.WriteString("\n\n")
	b.WriteString(renderEmailCheck(m.report.SPF, w))
	b.WriteString(renderEmailCheck(m.report.DMARC, w))
	for _, d := range m.report.DKIM {
		b.WriteString(renderEmailCheck(d, w))
	}
	return b.String()
}

func renderEmailCheck(c email.CheckResult, width int) string {
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
	b.WriteString(wrapLine(fmt.Sprintf("  %s %s — %s",
		st.Render(strings.ToUpper(string(c.Status))),
		c.Name,
		c.Summary,
	), width))
	b.WriteString("\n")
	for _, d := range c.Details {
		b.WriteString(muteStyle.Render(wrapLine("    · "+d, width)))
		b.WriteString("\n")
	}
	return b.String()
}

func utf8Len(s string) int {
	return len([]rune(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
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
