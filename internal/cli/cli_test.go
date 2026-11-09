package cli

import (
	"testing"

	"github.com/garymevs/mevs-dns-checker/internal/dns"
)

func TestParseInteractive(t *testing.T) {
	cfg, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Interactive || cfg.Domain != "" {
		t.Fatalf("expected interactive, got %+v", cfg)
	}
}

func TestParseDomainAndFlags(t *testing.T) {
	cfg, err := Parse([]string{"example.com", "-t", "MX"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Domain != "example.com" || cfg.Type != dns.TypeMX {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestParseEmailType(t *testing.T) {
	cfg, err := Parse([]string{"example.com", "-t", "EMAIL"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Type != dns.TypeEMAIL {
		t.Fatalf("type: %s", cfg.Type)
	}
}

func TestParseDKIMSelectorSelectsEmail(t *testing.T) {
	cfg, err := Parse([]string{"example.com", "-dkim-selector", "google,s1"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Type != dns.TypeEMAIL {
		t.Fatalf("expected EMAIL, got %s", cfg.Type)
	}
	if len(cfg.DKIMSelectors) != 2 || cfg.DKIMSelectors[0] != "google" {
		t.Fatalf("selectors: %+v", cfg.DKIMSelectors)
	}
}

func TestParseBadType(t *testing.T) {
	_, err := Parse([]string{"example.com", "-type", "PTR"})
	if err == nil {
		t.Fatal("expected error")
	}
}
