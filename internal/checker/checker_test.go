package checker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tagokoro/go-importflow/internal/config"
)

func TestCheckFindsLayerViolation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeFile(t, root, "internal/domain/user.go", "package domain\n\ntype User struct{}\n")
	writeFile(t, root, "internal/usecase/user.go", `package usecase

import "example.com/app/internal/infrastructure/postgres"

func Do() { postgres.Save() }
`)
	writeFile(t, root, "internal/infrastructure/postgres/store.go", `package postgres

import "example.com/app/internal/domain"

func Save() { _ = domain.User{} }
`)

	cfg := config.Config{Layers: []config.Layer{
		{Name: "domain", Patterns: []string{"internal/domain/**"}},
		{Name: "usecase", Patterns: []string{"internal/usecase/**"}, DependsOn: []string{"domain"}},
		{Name: "infrastructure", Patterns: []string{"internal/infrastructure/**"}, DependsOn: []string{"usecase", "domain"}},
	}}

	result, err := Check(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(result.Violations); got != 1 {
		t.Fatalf("expected 1 violation, got %d: %#v", got, result.Violations)
	}
	v := result.Violations[0]
	if v.FromLayer != "usecase" || v.ToLayer != "infrastructure" {
		t.Fatalf("unexpected violation: %#v", v)
	}
	if v.Line != 3 {
		t.Fatalf("expected import line 3, got %d", v.Line)
	}
}

func TestCheckAllowsConfiguredDependency(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeFile(t, root, "internal/domain/user.go", "package domain\n\ntype User struct{}\n")
	writeFile(t, root, "internal/usecase/user.go", `package usecase

import "example.com/app/internal/domain"

func Do() { _ = domain.User{} }
`)

	cfg := config.Config{Layers: []config.Layer{
		{Name: "domain", Patterns: []string{"internal/domain/**"}},
		{Name: "usecase", Patterns: []string{"internal/usecase/**"}, DependsOn: []string{"domain"}},
	}}

	result, err := Check(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(result.Violations); got != 0 {
		t.Fatalf("expected no violations, got %d: %#v", got, result.Violations)
	}
}

func TestCheckIgnoresConfiguredViolation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeFile(t, root, "internal/usecase/legacy/user.go", `package legacy

import "example.com/app/internal/infrastructure/postgres"

func Do() { postgres.Save() }
`)
	writeFile(t, root, "internal/infrastructure/postgres/store.go", "package postgres\n\nfunc Save() {}\n")

	cfg := config.Config{
		Layers: []config.Layer{
			{Name: "usecase", Patterns: []string{"internal/usecase/**"}},
			{Name: "infrastructure", Patterns: []string{"internal/infrastructure/**"}},
		},
		Ignores: []config.Ignore{
			{
				FromLayer: "usecase",
				ToLayer:   "infrastructure",
				File:      "internal/usecase/legacy/**",
				Reason:    "legacy migration",
			},
		},
	}

	result, err := Check(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(result.Violations); got != 0 {
		t.Fatalf("expected no active violations, got %d: %#v", got, result.Violations)
	}
	if got := len(result.IgnoredViolations); got != 1 {
		t.Fatalf("expected 1 ignored violation, got %d: %#v", got, result.IgnoredViolations)
	}
	if result.IgnoredViolations[0].Reason != "legacy migration" {
		t.Fatalf("unexpected ignored reason: %#v", result.IgnoredViolations[0])
	}
}

func TestCheckCanDisallowSameLayerDependency(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeFile(t, root, "internal/domain/user.go", "package domain\n\ntype User struct{}\n")
	writeFile(t, root, "internal/domain/service/service.go", `package service

import "example.com/app/internal/domain"

func New() domain.User { return domain.User{} }
`)

	allowSame := false
	cfg := config.Config{
		AllowSameLayer: &allowSame,
		Layers: []config.Layer{
			{Name: "domain", Patterns: []string{"internal/domain/**"}},
		},
	}

	result, err := Check(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(result.Violations); got != 1 {
		t.Fatalf("expected same-layer violation, got %d: %#v", got, result.Violations)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"internal/domain/**", "internal/domain", true},
		{"internal/domain/**", "internal/domain/model", true},
		{"internal/*/model", "internal/domain/model", true},
		{"cmd/**", "internal/cmd", false},
		{"", "", true},
	}

	for _, tt := range tests {
		if got := matchPattern(tt.pattern, tt.value); got != tt.want {
			t.Fatalf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
