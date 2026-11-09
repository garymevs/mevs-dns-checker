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
		prefix := fmt.Sprintf("%-18s %-10s %-8s ", r.Resolver.Name, r.Resolver.Region, rtt)
		cont := strings.Repeat(" ", len(prefix))
		switch {
		case r.HasError():
			fmt.Fprintf(w, "%sERR: %s\n", prefix, r.Err.Error())
		case len(r.Values) == 0:
			fmt.Fprintf(w, "%s(no records)\n", prefix)
		default:
			for i, v := range r.Values {
				if i == 0 {
					fmt.Fprintf(w, "%s%s\n", prefix, v)
				} else {
					fmt.Fprintf(w, "%s%s\n", cont, v)
				}
			}
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Answer groups:")
	for i, g := range sum.Consensus() {
		fmt.Fprintf(w, "  [%d] %d resolver(s)\n", i+1, g.Count)
		if g.Error != "" {
			fmt.Fprintf(w, "      ERR: %s\n", g.Error)
		} else if len(g.Values) == 0 {
			fmt.Fprintln(w, "      (empty)")
		} else {
			for _, v := range g.Values {
				fmt.Fprintf(w, "      · %s\n", v)
			}
		}
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
