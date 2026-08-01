package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tagokoro/go-dep-boundary/internal/checker"
	"github.com/tagokoro/go-dep-boundary/internal/config"
)

const (
	exitOK         = 0
	exitViolations = 1
	exitError      = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("depbound", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		configPath string
		root       string
		format     string
		initConfig bool
	)

	fs.StringVar(&configPath, "config", "depbound.json", "path to dependency boundary config")
	fs.StringVar(&root, "root", ".", "Go module root")
	fs.StringVar(&format, "format", "text", "output format: text or json")
	fs.BoolVar(&initConfig, "init", false, "write a sample config to -config")

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if initConfig {
		if err := config.WriteExample(configPath); err != nil {
			fmt.Fprintf(stderr, "depbound: write config: %v\n", err)
			return exitError
		}
		fmt.Fprintf(stdout, "wrote %s\n", configPath)
		return exitOK
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "depbound: resolve root: %v\n", err)
		return exitError
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "depbound: load config: %v\n", err)
		return exitError
	}

	result, err := checker.Check(rootAbs, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "depbound: check: %v\n", err)
		return exitError
	}

	switch format {
	case "text":
		printText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "depbound: encode json: %v\n", err)
			return exitError
		}
	default:
		fmt.Fprintf(stderr, "depbound: unknown -format %q\n", format)
		return exitError
	}

	if len(result.Violations) > 0 {
		return exitViolations
	}
	return exitOK
}

func printText(w io.Writer, result checker.Result) {
	if len(result.Violations) == 0 {
		if len(result.IgnoredViolations) > 0 {
			fmt.Fprintf(w, "depbound: ok (%d packages checked, %d ignored)\n", result.CheckedPackages, len(result.IgnoredViolations))
			return
		}
		fmt.Fprintf(w, "depbound: ok (%d packages checked)\n", result.CheckedPackages)
		return
	}

	fmt.Fprintf(w, "depbound: found %d dependency boundary violation(s)\n", len(result.Violations))
	for _, v := range result.Violations {
		loc := v.File
		if v.Line > 0 {
			loc = fmt.Sprintf("%s:%d", loc, v.Line)
		}
		fmt.Fprintf(
			w,
			"- %s [%s] imports %s [%s] at %s\n",
			v.FromPackage,
			v.FromLayer,
			v.ToPackage,
			v.ToLayer,
			loc,
		)
		fmt.Fprintf(w, "  rule: layer %q allowed dependencies: %s\n", v.FromLayer, joinOrNone(v.AllowedLayers))
	}
	if len(result.IgnoredViolations) > 0 {
		fmt.Fprintf(w, "depbound: ignored %d violation(s)\n", len(result.IgnoredViolations))
	}
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	out := values[0]
	for _, v := range values[1:] {
		out += ", " + v
	}
	return out
}
