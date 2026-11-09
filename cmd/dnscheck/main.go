package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/garymevs/mevs-dns-checker/internal/cli"
	"github.com/garymevs/mevs-dns-checker/internal/dns"
	"github.com/garymevs/mevs-dns-checker/internal/email"
	"github.com/garymevs/mevs-dns-checker/internal/tui"
	"github.com/garymevs/mevs-dns-checker/internal/version"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\n%s", err, cli.Usage())
		os.Exit(2)
	}
	if cfg.Help {
		fmt.Fprint(os.Stdout, cli.Usage())
		return
	}
	if cfg.ShowVersion {
		fmt.Fprintln(os.Stdout, version.Version)
		return
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	opts := tui.Options{
		Domain:        cfg.Domain,
		Type:          cfg.Type,
		DKIMSelectors: cfg.DKIMSelectors,
		Timeout:       timeout,
	}

	if cfg.NoTUI {
		if cfg.Domain == "" {
			fmt.Fprintln(os.Stderr, "error: -plain requires a domain argument")
			os.Exit(2)
		}
		if err := runPlain(opts); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := tui.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runPlain(opts tui.Options) error {
	client := dns.NewClient()
	client.Timeout = opts.Timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if opts.Type == dns.TypeEMAIL {
		r := email.NewChecker(client).Run(ctx, opts.Domain, opts.DKIMSelectors)
		tui.WritePlainEmail(os.Stdout, opts.Domain, &r)
		return nil
	}

	sum := client.Lookup(ctx, opts.Domain, opts.Type)
	tui.WritePlain(os.Stdout, sum, nil)
	return nil
}
