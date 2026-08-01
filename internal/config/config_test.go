package config

import "testing"

func TestValidateRejectsUnknownAllowedLayer(t *testing.T) {
	cfg := Config{Layers: []Layer{
		{Name: "usecase", Patterns: []string{"internal/usecase/**"}, DependsOn: []string{"domain"}},
	}}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSameLayerAllowedDefaultsToTrue(t *testing.T) {
	if !(Config{}).SameLayerAllowed() {
		t.Fatal("expected same-layer dependencies to be allowed by default")
	}
}

func TestValidateRejectsIgnoreWithoutSelector(t *testing.T) {
	cfg := Config{Layers: []Layer{
		{Name: "domain", Patterns: []string{"internal/domain/**"}},
	}, Ignores: []Ignore{{Reason: "too broad"}}}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRejectsIgnoreUnknownLayer(t *testing.T) {
	cfg := Config{Layers: []Layer{
		{Name: "domain", Patterns: []string{"internal/domain/**"}},
	}, Ignores: []Ignore{{FromLayer: "usecase"}}}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
