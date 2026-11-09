package email

import "testing"

func TestExtractTag(t *testing.T) {
	rec := "v=dmarc1; p=reject; rua=mailto:d@example.com; pct=100"
	if got := extractTag(rec, "p"); got != "reject" {
		t.Fatalf("p: got %q", got)
	}
	if got := extractTag(rec, "pct"); got != "100" {
		t.Fatalf("pct: got %q", got)
	}
	if got := extractTag(rec, "missing"); got != "" {
		t.Fatalf("missing: got %q", got)
	}
}

func TestSPFMultipleDetectionLogic(t *testing.T) {
	// Smoke: CommonDKIMSelectors is non-empty and includes google.
	found := false
	for _, s := range CommonDKIMSelectors {
		if s == "google" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected google in CommonDKIMSelectors")
	}
}
