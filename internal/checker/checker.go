package checker

import (
	"bufio"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tagokoro/go-importflow/internal/config"
)

type Result struct {
	CheckedPackages   int                `json:"checkedPackages"`
	Violations        []Violation        `json:"violations"`
	IgnoredViolations []IgnoredViolation `json:"ignoredViolations,omitempty"`
}

type Violation struct {
	FromPackage   string   `json:"fromPackage"`
	FromLayer     string   `json:"fromLayer"`
	ToPackage     string   `json:"toPackage"`
	ToLayer       string   `json:"toLayer"`
	ImportPath    string   `json:"importPath"`
	File          string   `json:"file"`
	RelFile       string   `json:"relFile"`
	Line          int      `json:"line"`
	AllowedLayers []string `json:"allowedLayers"`
	Rule          string   `json:"rule"`
}

type IgnoredViolation struct {
	Violation
	Reason string `json:"reason,omitempty"`
}

type goPackage struct {
	ImportPath string
	RelDir     string
	Dir        string
	Imports    []goImport
}

type goImport struct {
	Path    string
	File    string
	RelFile string
	Line    int
}

type layeredPackage struct {
	pkg   goPackage
	layer *config.Layer
}

func Check(root string, cfg config.Config) (Result, error) {
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}

	modulePath := cfg.Module
	if modulePath == "" {
		var err error
		modulePath, err = readModulePath(filepath.Join(root, "go.mod"))
		if err != nil {
			return Result{}, err
		}
	}

	packages, err := scanPackages(root, modulePath, cfg)
	if err != nil {
		return Result{}, err
	}

	byImportPath := map[string]layeredPackage{}
	for _, pkg := range packages {
		layer := findLayer(pkg.RelDir, cfg.Layers)
		if layer == nil {
			continue
		}
		byImportPath[pkg.ImportPath] = layeredPackage{pkg: pkg, layer: layer}
	}

	var violations []Violation
	var ignored []IgnoredViolation
	for _, src := range byImportPath {
		for _, imp := range src.pkg.Imports {
			dst, ok := byImportPath[imp.Path]
			if !ok {
				continue
			}
			violation := Violation{
				FromPackage:   src.pkg.ImportPath,
				FromLayer:     src.layer.Name,
				ToPackage:     dst.pkg.ImportPath,
				ToLayer:       dst.layer.Name,
				ImportPath:    imp.Path,
				File:          imp.File,
				RelFile:       imp.RelFile,
				Line:          imp.Line,
				AllowedLayers: src.layer.DependsOn,
			}
			if allowed(src.layer, dst.layer, cfg.SameLayerAllowed()) {
				continue
			}
			violation.Rule = "layer"
			if ignore, ok := matchingIgnore(violation, cfg.Ignores); ok {
				ignored = append(ignored, IgnoredViolation{Violation: violation, Reason: ignore.Reason})
				continue
			}
			violations = append(violations, violation)
		}
	}
	sortViolations(violations)
	sort.Slice(ignored, func(i, j int) bool {
		return compareViolations(ignored[i].Violation, ignored[j].Violation) < 0
	})

	return Result{
		CheckedPackages:   len(byImportPath),
		Violations:        violations,
		IgnoredViolations: ignored,
	}, nil
}

func readModulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read module path from go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("module directive not found in go.mod")
}

func scanPackages(root string, modulePath string, cfg config.Config) ([]goPackage, error) {
	packages := map[string]*goPackage{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			name := d.Name()
			if shouldSkipDir(name, path, root, cfg.IgnoreDirs) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		if !cfg.IncludeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		relDir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		relDir = filepath.ToSlash(relDir)
		if relDir == "." {
			relDir = ""
		}

		importPath := modulePath
		if relDir != "" {
			importPath += "/" + relDir
		}

		pkg := packages[importPath]
		if pkg == nil {
			pkg = &goPackage{
				ImportPath: importPath,
				RelDir:     relDir,
				Dir:        filepath.Dir(path),
			}
			packages[importPath] = pkg
		}

		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("parse import in %s: %w", path, err)
			}
			relFile, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			pkg.Imports = append(pkg.Imports, goImport{
				Path:    importPath,
				File:    path,
				RelFile: filepath.ToSlash(relFile),
				Line:    fset.Position(spec.Pos()).Line,
			})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]goPackage, 0, len(packages))
	for _, pkg := range packages {
		out = append(out, *pkg)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ImportPath < out[j].ImportPath
	})
	return out, nil
}

func shouldSkipDir(name string, path string, root string, ignoreDirs []string) bool {
	if name == "vendor" || name == ".git" {
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, ignored := range ignoreDirs {
		if ignored == name || matchPattern(ignored, rel) {
			return true
		}
	}
	return false
}

func findLayer(relDir string, layers []config.Layer) *config.Layer {
	for i := range layers {
		for _, pattern := range layers[i].Patterns {
			if matchPattern(pattern, relDir) {
				return &layers[i]
			}
		}
	}
	return nil
}

func allowed(src *config.Layer, dst *config.Layer, allowSameLayer bool) bool {
	if src == nil || dst == nil {
		return true
	}
	if src.Name == dst.Name {
		return allowSameLayer
	}
	for _, allowed := range src.DependsOn {
		if allowed == dst.Name {
			return true
		}
	}
	return false
}

func matchingIgnore(v Violation, ignores []config.Ignore) (config.Ignore, bool) {
	for _, ignore := range ignores {
		if matchesIgnore(v, ignore) {
			return ignore, true
		}
	}
	return config.Ignore{}, false
}

func matchesIgnore(v Violation, ignore config.Ignore) bool {
	return matchOptional(ignore.FromLayer, v.FromLayer) &&
		matchOptional(ignore.ToLayer, v.ToLayer) &&
		matchOptional(ignore.FromPackage, v.FromPackage) &&
		matchOptional(ignore.ToPackage, v.ToPackage) &&
		matchOptional(ignore.ImportPath, v.ImportPath) &&
		matchOptional(ignore.File, v.RelFile)
}

func matchOptional(pattern string, value string) bool {
	if pattern == "" {
		return true
	}
	return pattern == value || matchPattern(pattern, value)
}

func sortViolations(violations []Violation) {
	sort.Slice(violations, func(i, j int) bool {
		return compareViolations(violations[i], violations[j]) < 0
	})
}

func compareViolations(a Violation, b Violation) int {
	if a.File != b.File {
		return strings.Compare(a.File, b.File)
	}
	if a.Line != b.Line {
		if a.Line < b.Line {
			return -1
		}
		return 1
	}
	if a.FromPackage != b.FromPackage {
		return strings.Compare(a.FromPackage, b.FromPackage)
	}
	return strings.Compare(a.ToPackage, b.ToPackage)
}
