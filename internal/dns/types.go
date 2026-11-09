package dns

import (
	"sort"
	"strings"
	"time"

	"github.com/garymevs/mevs-dns-checker/internal/resolvers"
)

// RecordType is a DNS RR type the checker supports.
type RecordType string

const (
	TypeA     RecordType = "A"
	TypeAAAA  RecordType = "AAAA"
	TypeCNAME RecordType = "CNAME"
	TypeMX    RecordType = "MX"
	TypeNS    RecordType = "NS"
	TypeTXT   RecordType = "TXT"
	TypeSOA   RecordType = "SOA"
	TypeEMAIL RecordType = "EMAIL" // SPF / DKIM / DMARC suite
)

// SupportedTypes is the MVP record-type set (includes EMAIL auth tab).
var SupportedTypes = []RecordType{
	TypeA, TypeAAAA, TypeCNAME, TypeMX, TypeNS, TypeTXT, TypeEMAIL,
}

// IsDNSType reports whether t is a normal RR lookup (not EMAIL).
func (t RecordType) IsDNSType() bool {
	return t != TypeEMAIL && t != ""
}

// Description returns a short explanation of what this type checks.
func (t RecordType) Description() string {
	switch t {
	case TypeA:
		return "IPv4 addresses for the host"
	case TypeAAAA:
		return "IPv6 addresses for the host"
	case TypeCNAME:
		return "Canonical name alias pointing at another hostname"
	case TypeMX:
		return "Mail exchange servers that receive email for the domain"
	case TypeNS:
		return "Authoritative name servers for the domain"
	case TypeTXT:
		return "Arbitrary text records (verification, policies, etc.)"
	case TypeEMAIL:
		return "SPF, DKIM, and DMARC — email authentication / spoofing protection"
	case TypeSOA:
		return "Start of authority — zone primary and timing parameters"
	default:
		return ""
	}
}

// ParseRecordType normalizes a user-supplied type string.
func ParseRecordType(s string) (RecordType, bool) {
	t := RecordType(strings.ToUpper(strings.TrimSpace(s)))
	switch t {
	case "AUTH", "SPF", "DKIM", "DMARC":
		return TypeEMAIL, true
	}
	for _, supported := range SupportedTypes {
		if t == supported {
			return t, true
		}
	}
	return "", false
}

// Result is one resolver's answer for a query.
type Result struct {
	Resolver resolvers.Resolver
	Values   []string
	TTL      uint32
	RTT      time.Duration
	Err      error
}

// HasError reports whether the query failed.
func (r Result) HasError() bool {
	return r.Err != nil
}

// Summary aggregates multi-resolver results for consensus display.
type Summary struct {
	Domain  string
	Type    RecordType
	Results []Result
}

// Consensus groups identical answer sets and counts how many resolvers agree.
func (s Summary) Consensus() []AnswerGroup {
	groups := map[string]*AnswerGroup{}
	order := []string{}

	for _, r := range s.Results {
		if r.HasError() {
			key := "__error__:" + r.Err.Error()
			g, ok := groups[key]
			if !ok {
				g = &AnswerGroup{Values: nil, Error: r.Err.Error()}
				groups[key] = g
				order = append(order, key)
			}
			g.Resolvers = append(g.Resolvers, r.Resolver.Name)
			g.Count++
			continue
		}

		key := strings.Join(normalizeValues(r.Values), "\n")
		g, ok := groups[key]
		if !ok {
			g = &AnswerGroup{Values: append([]string(nil), r.Values...)}
			groups[key] = g
			order = append(order, key)
		}
		g.Resolvers = append(g.Resolvers, r.Resolver.Name)
		g.Count++
	}

	out := make([]AnswerGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *groups[key])
	}
	return out
}

// AnswerGroup is a set of identical answers across resolvers.
type AnswerGroup struct {
	Values    []string
	Error     string
	Resolvers []string
	Count     int
}

// IsAgreement is true when every successful resolver returned the same values
// and there were no errors.
func (s Summary) IsAgreement() bool {
	groups := s.Consensus()
	if len(groups) != 1 {
		return false
	}
	return groups[0].Error == "" && groups[0].Count == len(s.Results)
}

func normalizeValues(values []string) []string {
	out := append([]string(nil), values...)
	for i := range out {
		out[i] = strings.TrimSuffix(strings.ToLower(out[i]), ".")
	}
	sort.Strings(out)
	return out
}
