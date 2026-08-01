package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	Module         string   `json:"module"`
	AllowSameLayer *bool    `json:"allowSameLayer,omitempty"`
	IncludeTests   bool     `json:"includeTests,omitempty"`
	IgnoreDirs     []string `json:"ignoreDirs,omitempty"`
	Layers         []Layer  `json:"layers"`
	Ignores        []Ignore `json:"ignores,omitempty"`
}

type Layer struct {
	Name      string   `json:"name"`
	Patterns  []string `json:"patterns"`
	DependsOn []string `json:"dependsOn"`
}

type Ignore struct {
	FromLayer   string `json:"fromLayer,omitempty"`
	ToLayer     string `json:"toLayer,omitempty"`
	FromPackage string `json:"fromPackage,omitempty"`
	ToPackage   string `json:"toPackage,omitempty"`
	ImportPath  string `json:"importPath,omitempty"`
	File        string `json:"file,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if len(c.Layers) == 0 {
		return errors.New("at least one layer is required")
	}

	seen := map[string]struct{}{}
	for i, layer := range c.Layers {
		if layer.Name == "" {
			return fmt.Errorf("layers[%d].name is required", i)
		}
		if len(layer.Patterns) == 0 {
			return fmt.Errorf("layers[%d].patterns is required", i)
		}
		if _, ok := seen[layer.Name]; ok {
			return fmt.Errorf("duplicate layer %q", layer.Name)
		}
		seen[layer.Name] = struct{}{}
	}

	for _, layer := range c.Layers {
		for _, allowed := range layer.DependsOn {
			if _, ok := seen[allowed]; !ok {
				return fmt.Errorf("layer %q allows unknown layer %q", layer.Name, allowed)
			}
		}
	}

	for i, ignore := range c.Ignores {
		if !ignore.HasSelector() {
			return fmt.Errorf("ignores[%d] must set at least one selector", i)
		}
		if ignore.FromLayer != "" {
			if _, ok := seen[ignore.FromLayer]; !ok {
				return fmt.Errorf("ignores[%d].fromLayer references unknown layer %q", i, ignore.FromLayer)
			}
		}
		if ignore.ToLayer != "" {
			if _, ok := seen[ignore.ToLayer]; !ok {
				return fmt.Errorf("ignores[%d].toLayer references unknown layer %q", i, ignore.ToLayer)
			}
		}
	}

	return nil
}

func (i Ignore) HasSelector() bool {
	return i.FromLayer != "" ||
		i.ToLayer != "" ||
		i.FromPackage != "" ||
		i.ToPackage != "" ||
		i.ImportPath != "" ||
		i.File != ""
}

func (c Config) SameLayerAllowed() bool {
	if c.AllowSameLayer == nil {
		return true
	}
	return *c.AllowSameLayer
}

func WriteExample(path string) error {
	return os.WriteFile(path, []byte(exampleConfig), 0644)
}

const exampleConfig = `{
  "module": "",
  "allowSameLayer": true,
  "includeTests": false,
  "ignoreDirs": ["vendor", ".git"],
  "layers": [
    {
      "name": "domain",
      "patterns": ["internal/domain/**"],
      "dependsOn": []
    },
    {
      "name": "usecase",
      "patterns": ["internal/usecase/**"],
      "dependsOn": ["domain"]
    },
    {
      "name": "interface",
      "patterns": ["internal/interface/**", "internal/adapter/**"],
      "dependsOn": ["usecase", "domain"]
    },
    {
      "name": "infrastructure",
      "patterns": ["internal/infrastructure/**", "cmd/**"],
      "dependsOn": ["interface", "usecase", "domain"]
    }
  ],
  "ignores": [
    {
      "fromLayer": "usecase",
      "toLayer": "infrastructure",
      "file": "internal/usecase/legacy/**",
      "reason": "legacy migration"
    }
  ]
}
`
