package dns

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/garymevs/mevs-dns-checker/internal/resolvers"
	mdns "github.com/miekg/dns"
)

// Client queries multiple public resolvers concurrently.
type Client struct {
	Resolvers []resolvers.Resolver
	Timeout   time.Duration
}

// NewClient builds a client with the default resolver set.
func NewClient() *Client {
	return &Client{
		Resolvers: resolvers.Default(),
		Timeout:   3 * time.Second,
	}
}

// Lookup queries every configured resolver for domain/type.
func (c *Client) Lookup(ctx context.Context, domain string, rtype RecordType) Summary {
	domain = normalizeDomain(domain)
	results := make([]Result, len(c.Resolvers))

	var wg sync.WaitGroup
	for i, r := range c.Resolvers {
		wg.Add(1)
		go func(i int, r resolvers.Resolver) {
			defer wg.Done()
			results[i] = c.queryOne(ctx, r, domain, rtype)
		}(i, r)
	}
	wg.Wait()

	return Summary{
		Domain:  domain,
		Type:    rtype,
		Results: results,
	}
}

// LookupOne queries a single resolver (used by email checks for authoritative TXT).
func (c *Client) LookupOne(ctx context.Context, resolver resolvers.Resolver, domain string, rtype RecordType) Result {
	return c.queryOne(ctx, resolver, normalizeDomain(domain), rtype)
}

// LookupTXT collects unique TXT strings across resolvers (best-effort).
func (c *Client) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	sum := c.Lookup(ctx, domain, TypeTXT)
	seen := map[string]struct{}{}
	var out []string
	var firstErr error
	for _, r := range sum.Results {
		if r.HasError() {
			if firstErr == nil {
				firstErr = r.Err
			}
			continue
		}
		for _, v := range r.Values {
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	sort.Strings(out)
	return out, nil
}

func (c *Client) queryOne(ctx context.Context, resolver resolvers.Resolver, domain string, rtype RecordType) Result {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	qtype, ok := toDNSType(rtype)
	if !ok {
		return Result{Resolver: resolver, Err: fmt.Errorf("unsupported type %s", rtype)}
	}

	msg := new(mdns.Msg)
	msg.SetQuestion(mdns.Fqdn(domain), qtype)
	msg.RecursionDesired = true
	msg.SetEdns0(4096, false)

	client := &mdns.Client{
		Net:     "udp",
		Timeout: timeout,
		UDPSize: 4096,
		Dialer:  &net.Dialer{Timeout: timeout},
	}

	start := time.Now()
	resp, rtt, err := client.ExchangeContext(ctx, msg, resolver.Address)
	if err != nil {
		return Result{Resolver: resolver, RTT: time.Since(start), Err: err}
	}
	if resp == nil {
		return Result{Resolver: resolver, RTT: rtt, Err: fmt.Errorf("empty response")}
	}

	// Large TXT/RRsets often exceed UDP; retry over TCP when truncated.
	if resp.Truncated {
		tcpClient := &mdns.Client{
			Net:     "tcp",
			Timeout: timeout,
			Dialer:  &net.Dialer{Timeout: timeout},
		}
		tcpResp, tcpRTT, tcpErr := tcpClient.ExchangeContext(ctx, msg, resolver.Address)
		if tcpErr == nil && tcpResp != nil {
			resp, rtt = tcpResp, rtt+tcpRTT
		}
	}

	if resp.Rcode != mdns.RcodeSuccess {
		return Result{
			Resolver: resolver,
			RTT:      rtt,
			Err:      fmt.Errorf("%s", mdns.RcodeToString[resp.Rcode]),
		}
	}

	values, ttl := formatAnswers(resp.Answer, rtype)
	return Result{
		Resolver: resolver,
		Values:   values,
		TTL:      ttl,
		RTT:      rtt,
	}
}

func toDNSType(t RecordType) (uint16, bool) {
	switch t {
	case TypeA:
		return mdns.TypeA, true
	case TypeAAAA:
		return mdns.TypeAAAA, true
	case TypeCNAME:
		return mdns.TypeCNAME, true
	case TypeMX:
		return mdns.TypeMX, true
	case TypeNS:
		return mdns.TypeNS, true
	case TypeTXT:
		return mdns.TypeTXT, true
	case TypeSOA:
		return mdns.TypeSOA, true
	default:
		return 0, false
	}
}

func formatAnswers(rrs []mdns.RR, rtype RecordType) ([]string, uint32) {
	var values []string
	var ttl uint32
	for _, rr := range rrs {
		if ttl == 0 {
			ttl = rr.Header().Ttl
		}
		switch v := rr.(type) {
		case *mdns.A:
			if rtype == TypeA {
				values = append(values, v.A.String())
			}
		case *mdns.AAAA:
			if rtype == TypeAAAA {
				values = append(values, v.AAAA.String())
			}
		case *mdns.CNAME:
			if rtype == TypeCNAME {
				values = append(values, strings.TrimSuffix(v.Target, "."))
			}
		case *mdns.MX:
			if rtype == TypeMX {
				values = append(values, fmt.Sprintf("%d %s", v.Preference, strings.TrimSuffix(v.Mx, ".")))
			}
		case *mdns.NS:
			if rtype == TypeNS {
				values = append(values, strings.TrimSuffix(v.Ns, "."))
			}
		case *mdns.TXT:
			if rtype == TypeTXT {
				values = append(values, strings.Join(v.Txt, ""))
			}
		case *mdns.SOA:
			if rtype == TypeSOA {
				values = append(values, fmt.Sprintf("%s %s %d %d %d %d %d",
					strings.TrimSuffix(v.Ns, "."),
					strings.TrimSuffix(v.Mbox, "."),
					v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl,
				))
			}
		}
	}
	sort.Strings(values)
	return values, ttl
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
}
