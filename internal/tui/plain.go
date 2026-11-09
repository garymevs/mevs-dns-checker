package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/garymevs/mevs-dns-checker/internal/dns"
	"github.com/garymevs/mevs-dns-checker/internal/email"
)

// WritePlain renders multi-resolver DNS results as plain text.
func WritePlain(w io.Writer, sum dns.Summary, _ *email.Report) {
	fmt.Fprintf(w, "DNS check: %s (%s)\n", sum.Domain, sum.Type)
	fmt.Fprintln(w, strings.Repeat("=", 60))

	if sum.IsAgreement() {
		fmt.Fprintln(w, "Consensus: all resolvers agree")
	} else {
		fmt.Fprintln(w, "Consensus: DIFFERENCES detected")
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "%-18s %-10s %-8s %s\n", "RESOLVER", "REGION", "RTT", "RESULT")
	fmt.Fprintln(w, strings.Repeat("-", 60))
	for _, r := range sum.Results {
		rtt := formatRTT(r.RTT)
		var result string
		if r.HasError() {
			result = "ERR: " + r.Err.Error()
		} else if len(r.Values) == 0 {
			result = "(no records)"
		} else {
			result = strings.Join(r.Values, ", ")
		}
		fmt.Fprintf(w, "%-18s %-10s %-8s %s\n", r.Resolver.Name, r.Resolver.Region, rtt, result)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Answer groups:")
	for i, g := range sum.Consensus() {
		label := strings.Join(g.Values, ", ")
		if g.Error != "" {
			label = "ERR: " + g.Error
		} else if label == "" {
			label = "(empty)"
		}
		fmt.Fprintf(w, "  [%d] %d resolver(s): %s\n", i+1, g.Count, label)
		fmt.Fprintf(w, "      %s\n", strings.Join(g.Resolvers, ", "))
	}
}

// WritePlainEmail renders SPF / DKIM / DMARC results as plain text.
func WritePlainEmail(w io.Writer, domain string, report *email.Report) {
	fmt.Fprintf(w, "Email authentication: %s\n", domain)
	fmt.Fprintln(w, strings.Repeat("=", 60))
	if report == nil {
		fmt.Fprintln(w, "(no report)")
		return
	}
	writeEmailCheck(w, report.SPF)
	writeEmailCheck(w, report.DMARC)
	for _, d := range report.DKIM {
		writeEmailCheck(w, d)
	}
}

func writeEmailCheck(w io.Writer, c email.CheckResult) {
	fmt.Fprintf(w, "\n[%s] %s — %s\n", strings.ToUpper(string(c.Status)), c.Name, c.Summary)
	for _, d := range c.Details {
		fmt.Fprintf(w, "  - %s\n", d)
	}
}

func formatRTT(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return d.Round(time.Millisecond).String()
}
