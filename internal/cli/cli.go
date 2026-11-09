package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/garymevs/mevs-dns-checker/internal/dns"
)

// Config is the parsed CLI invocation.
type Config struct {
	Domain        string
	Type          dns.RecordType
	DKIMSelectors []string
	Interactive   bool
	TimeoutSec    int
	NoTUI         bool // plain text output
	Help          bool
	ShowVersion   bool
}

var valueFlags = map[string]struct{}{
	"-type":          {},
	"-t":             {},
	"-dkim-selector": {},
	"-timeout":       {},
}

// Parse reads os.Args into a Config.
// Flags may appear before or after the domain.
// No domain → Interactive.
func Parse(args []string) (Config, error) {
	domain, flagArgs, err := splitArgs(args)
	if err != nil {
		return Config{}, err
	}

	fs := flag.NewFlagSet("dnscheck", flag.ContinueOnError)
	var flagErr strings.Builder
	fs.SetOutput(&flagErr)

	var (
		typeStr   = fs.String("type", "A", "Type: A, AAAA, CNAME, MX, NS, TXT, EMAIL")
		typeShort = fs.String("t", "", "Shorthand for -type")
		dkim      = fs.String("dkim-selector", "", "Comma-separated DKIM selectors (EMAIL type)")
		timeout   = fs.Int("timeout", 3, "Per-resolver timeout in seconds")
		plain     = fs.Bool("plain", false, "Plain text output instead of TUI")
		ver       = fs.Bool("version", false, "Print version and exit")
		verShort  = fs.Bool("V", false, "Print version and exit")
		help      = fs.Bool("help", false, "Show help")
		helpShort = fs.Bool("h", false, "Show help")
	)

	if err := fs.Parse(flagArgs); err != nil {
		msg := strings.TrimSpace(flagErr.String())
		if msg != "" {
			return Config{}, fmt.Errorf("%s", msg)
		}
		return Config{}, err
	}
	if rest := fs.Args(); len(rest) > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", rest)
	}

	cfg := Config{
		Domain:      strings.TrimSpace(domain),
		TimeoutSec:  *timeout,
		NoTUI:       *plain,
		Help:        *help || *helpShort,
		ShowVersion: *ver || *verShort,
	}

	if cfg.Help || cfg.ShowVersion {
		return cfg, nil
	}

	t := *typeStr
	if *typeShort != "" {
		t = *typeShort
	}
	rt, ok := dns.ParseRecordType(t)
	if !ok {
		return Config{}, fmt.Errorf("unsupported type %q (use A, AAAA, CNAME, MX, NS, TXT, EMAIL)", t)
	}
	cfg.Type = rt

	if *dkim != "" {
		for _, s := range strings.Split(*dkim, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cfg.DKIMSelectors = append(cfg.DKIMSelectors, s)
			}
		}
		// Selectors only apply to the EMAIL tab.
		if cfg.Type != dns.TypeEMAIL {
			cfg.Type = dns.TypeEMAIL
		}
	}

	if cfg.Domain == "" {
		cfg.Interactive = true
	}

	return cfg, nil
}

func splitArgs(args []string) (domain string, flagArgs []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			for _, p := range args[i+1:] {
				if domain == "" {
					domain = p
				} else {
					return "", nil, fmt.Errorf("unexpected arguments: %v", args[i+1:])
				}
			}
			return domain, flagArgs, nil
		case strings.HasPrefix(a, "-"):
			flagArgs = append(flagArgs, a)
			name, hasEq := a, strings.Contains(a, "=")
			if !hasEq {
				if _, ok := valueFlags[name]; ok {
					if i+1 >= len(args) {
						return "", nil, fmt.Errorf("flag %s requires a value", name)
					}
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		default:
			if domain != "" {
				return "", nil, fmt.Errorf("unexpected arguments: %q", a)
			}
			domain = a
		}
	}
	return domain, flagArgs, nil
}

// Usage returns the help text.
func Usage() string {
	return `dnscheck — multi-resolver DNS checker (TUI)

Usage:
  dnscheck                         Interactive mode
  dnscheck <domain> [flags]        Check domain and show results

Flags:
  -t, -type string         A, AAAA, CNAME, MX, NS, TXT, EMAIL (default A)
  -dkim-selector string    DKIM selectors for EMAIL (comma-separated; selects EMAIL)
  -timeout int             Per-resolver timeout seconds (default 3)
  -plain                   Plain text output (no TUI)
  -V, -version             Print version
  -h, -help                Show help

Examples:
  dnscheck example.com
  dnscheck example.com -t MX
  dnscheck example.com -t EMAIL
  dnscheck example.com -dkim-selector google,selector1
`
}
