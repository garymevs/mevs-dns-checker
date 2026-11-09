// Package email implements SPF, DKIM, and DMARC DNS-based checks.
package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/garymevs/mevs-dns-checker/internal/dns"
)

// Status is a high-level check outcome.
type Status string

const (
	StatusPass    Status = "pass"
	StatusWarn    Status = "warn"
	StatusFail    Status = "fail"
	StatusMissing Status = "missing"
)

// CheckResult is one email-auth check.
type CheckResult struct {
	Name    string
	Status  Status
	Summary string
	Details []string
	Raw     []string
}

// Report holds SPF/DKIM/DMARC results for a domain.
type Report struct {
	Domain string
	SPF    CheckResult
	DMARC  CheckResult
	DKIM   []CheckResult
}

// CommonDKIMSelectors are tried when the user does not supply selectors.
var CommonDKIMSelectors = []string{
	"default",
	"google",
	"selector1",
	"selector2",
	"k1",
	"k2",
	"s1",
	"s2",
	"dkim",
	"mail",
	"smtp",
	"mandrill",
	"everlytickey1",
	"everlytickey2",
	"cm",
	"mx",
	"protonmail",
	"pm",
	"zoho",
	"sig1",
}

// Checker runs email authentication DNS lookups.
type Checker struct {
	DNS *dns.Client
}

// NewChecker creates a checker using the given DNS client.
func NewChecker(client *dns.Client) *Checker {
	return &Checker{DNS: client}
}

// Run performs SPF, DMARC, and DKIM checks.
// If selectors is empty, CommonDKIMSelectors are probed and only hits are kept.
func (c *Checker) Run(ctx context.Context, domain string, selectors []string) Report {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	report := Report{Domain: domain}
	report.SPF = c.checkSPF(ctx, domain)
	report.DMARC = c.checkDMARC(ctx, domain)
	report.DKIM = c.checkDKIM(ctx, domain, selectors)
	return report
}

func (c *Checker) checkSPF(ctx context.Context, domain string) CheckResult {
	txts, err := c.DNS.LookupTXT(ctx, domain)
	if err != nil {
		return CheckResult{
			Name:    "SPF",
			Status:  StatusFail,
			Summary: "TXT lookup failed",
			Details: []string{err.Error()},
		}
	}

	var spfRecords []string
	for _, t := range txts {
		if strings.HasPrefix(strings.ToLower(t), "v=spf1") {
			spfRecords = append(spfRecords, t)
		}
	}

	if len(spfRecords) == 0 {
		return CheckResult{
			Name:    "SPF",
			Status:  StatusMissing,
			Summary: "No SPF record found",
			Details: []string{"Publish a TXT record starting with v=spf1"},
		}
	}

	result := CheckResult{
		Name: "SPF",
		Raw:  spfRecords,
	}

	if len(spfRecords) > 1 {
		result.Status = StatusFail
		result.Summary = fmt.Sprintf("%d SPF records found (only one allowed)", len(spfRecords))
		result.Details = append(result.Details, "Multiple SPF TXT records invalidate SPF")
		return result
	}

	rec := spfRecords[0]
	lower := strings.ToLower(rec)
	result.Summary = "SPF record present"
	result.Status = StatusPass

	switch {
	case strings.Contains(lower, " -all"):
		result.Details = append(result.Details, "Policy: hard fail (-all)")
	case strings.Contains(lower, " ~all"):
		result.Status = StatusWarn
		result.Details = append(result.Details, "Policy: soft fail (~all) — consider -all when ready")
	case strings.Contains(lower, " ?all"):
		result.Status = StatusWarn
		result.Details = append(result.Details, "Policy: neutral (?all)")
	case strings.Contains(lower, " +all"):
		result.Status = StatusFail
		result.Summary = "SPF allows any sender (+all)"
		result.Details = append(result.Details, "Dangerous: +all permits spoofing")
	default:
		result.Status = StatusWarn
		result.Details = append(result.Details, "No clear all mechanism found")
	}

	if strings.Count(lower, "include:") > 10 {
		result.Status = StatusWarn
		result.Details = append(result.Details, "Many include: mechanisms — watch DNS lookup limits (10)")
	}

	result.Details = append(result.Details, rec)
	return result
}

func (c *Checker) checkDMARC(ctx context.Context, domain string) CheckResult {
	name := "_dmarc." + domain
	txts, err := c.DNS.LookupTXT(ctx, name)
	if err != nil {
		return CheckResult{
			Name:    "DMARC",
			Status:  StatusFail,
			Summary: "TXT lookup failed for _dmarc." + domain,
			Details: []string{err.Error()},
		}
	}

	var dmarc []string
	for _, t := range txts {
		if strings.HasPrefix(strings.ToLower(t), "v=dmarc1") {
			dmarc = append(dmarc, t)
		}
	}

	if len(dmarc) == 0 {
		return CheckResult{
			Name:    "DMARC",
			Status:  StatusMissing,
			Summary: "No DMARC record at _dmarc." + domain,
			Details: []string{"Publish TXT v=DMARC1; p=none|quarantine|reject"},
		}
	}

	result := CheckResult{Name: "DMARC", Raw: dmarc}
	if len(dmarc) > 1 {
		result.Status = StatusFail
		result.Summary = "Multiple DMARC records found"
		return result
	}

	rec := dmarc[0]
	lower := strings.ToLower(rec)
	result.Details = append(result.Details, rec)
	result.Summary = "DMARC record present"
	result.Status = StatusPass

	policy := extractTag(lower, "p")
	switch policy {
	case "reject":
		result.Details = append([]string{"Policy: reject"}, result.Details...)
	case "quarantine":
		result.Status = StatusWarn
		result.Details = append([]string{"Policy: quarantine"}, result.Details...)
	case "none":
		result.Status = StatusWarn
		result.Details = append([]string{"Policy: none (monitor only)"}, result.Details...)
	default:
		result.Status = StatusWarn
		result.Details = append([]string{"Missing or unknown p= policy"}, result.Details...)
	}

	if pct := extractTag(lower, "pct"); pct != "" && pct != "100" {
		result.Status = StatusWarn
		result.Details = append(result.Details, "pct="+pct+" — policy applied to a subset of mail")
	}

	return result
}

func (c *Checker) checkDKIM(ctx context.Context, domain string, selectors []string) []CheckResult {
	probeOnly := false
	if len(selectors) == 0 {
		selectors = CommonDKIMSelectors
		probeOnly = true
	}

	var found []CheckResult
	var missing []CheckResult

	for _, sel := range selectors {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}
		cr := c.lookupDKIMSelector(ctx, domain, sel)
		if cr.Status == StatusMissing || cr.Status == StatusFail {
			if !probeOnly {
				missing = append(missing, cr)
			}
			continue
		}
		found = append(found, cr)
	}

	if probeOnly {
		if len(found) == 0 {
			return []CheckResult{{
				Name:    "DKIM",
				Status:  StatusMissing,
				Summary: "No DKIM keys found for common selectors",
				Details: []string{
					"Tried: " + strings.Join(CommonDKIMSelectors, ", "),
					"Pass --dkim-selector <name> for your provider's selector",
				},
			}}
		}
		return found
	}

	return append(found, missing...)
}

func (c *Checker) lookupDKIMSelector(ctx context.Context, domain, selector string) CheckResult {
	name := selector + "._domainkey." + domain
	txts, err := c.DNS.LookupTXT(ctx, name)
	if err != nil {
		return CheckResult{
			Name:    "DKIM (" + selector + ")",
			Status:  StatusFail,
			Summary: "Lookup failed for " + name,
			Details: []string{err.Error()},
		}
	}

	var keys []string
	for _, t := range txts {
		lt := strings.ToLower(t)
		if strings.Contains(lt, "v=dkim1") || strings.Contains(lt, "p=") {
			keys = append(keys, t)
		}
	}

	if len(keys) == 0 {
		return CheckResult{
			Name:    "DKIM (" + selector + ")",
			Status:  StatusMissing,
			Summary: "No DKIM key at " + name,
		}
	}

	result := CheckResult{
		Name:    "DKIM (" + selector + ")",
		Status:  StatusPass,
		Summary: "DKIM key found",
		Raw:     keys,
		Details: []string{name},
	}

	for _, k := range keys {
		lower := strings.ToLower(k)
		if extractTag(lower, "p") == "" {
			result.Status = StatusFail
			result.Summary = "DKIM record missing p= public key"
		}
		if v := extractTag(lower, "v"); v != "" && v != "dkim1" {
			result.Status = StatusWarn
			result.Details = append(result.Details, "Unexpected v="+v)
		}
		display := k
		if len(display) > 120 {
			display = display[:117] + "..."
		}
		result.Details = append(result.Details, display)
	}

	return result
}

func extractTag(record, tag string) string {
	// record expected lowercased; tags are semicolon-separated
	parts := strings.Split(record, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !strings.Contains(p, "=") {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if strings.TrimSpace(kv[0]) == tag {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}
