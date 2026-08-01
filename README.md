# go-importflow

A CI-friendly CLI that statically checks Go import flow and reports dependency direction violations, such as Clean Architecture layer direction breaks.

## Features

- Reads only `go.mod` and `.go` files, so it does not need to build the project or resolve external dependencies.
- Exits with status `1` when dependency boundary violations are found, which makes it easy to use in CI.
- Defines layer rules in JSON.
- Can include or exclude `*_test.go` files.
- Supports scoped exceptions for legacy code or migration windows.

## Usage

Install:

```sh
go install github.com/tagokoro/go-importflow/cmd/go-importflow@latest
```

Run from source:

```sh
go run ./cmd/go-importflow -config go-importflow.json
```

When there are no violations:

```text
go-importflow: ok (4 packages checked)
```

When violations are found:

```text
go-importflow: found 1 dependency boundary violation(s)
- example.com/app/internal/usecase [usecase] imports example.com/app/internal/infrastructure/postgres [infrastructure] at /repo/internal/usecase/user.go:3
  rule: layer "usecase" allowed dependencies: domain
```

JSON output:

```sh
go run ./cmd/go-importflow -config go-importflow.json -format json
```

## Configuration

Generate a sample config:

```sh
go run ./cmd/go-importflow -init -config go-importflow.json
```

Example:

```json
{
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
  ]
}
```

If `module` is empty, go-importflow reads the module path from the target project's `go.mod`.

`patterns` are matched against package directories. `**` matches multiple path segments. For example, `internal/domain/**` matches both `internal/domain` and `internal/domain/model`.

`dependsOn` lists the layers that the current layer is allowed to import. Same-layer imports are allowed when `allowSameLayer` is `true`.

## Ignoring Exceptions

Use `ignores` when you need to suppress specific violations temporarily, for example during a legacy migration.

```json
{
  "ignores": [
    {
      "fromLayer": "usecase",
      "toLayer": "infrastructure",
      "file": "internal/usecase/legacy/**",
      "reason": "legacy migration"
    }
  ]
}
```

Supported selectors:

- `fromLayer`: source layer name
- `toLayer`: imported layer name
- `fromPackage`: source package, with glob support such as `example.com/app/internal/usecase/**`
- `toPackage`: imported package
- `importPath`: import path from the Go import declaration
- `file`: file containing the import, relative to the project root
- `reason`: explanation shown in `ignoredViolations` in JSON output

When multiple selectors are set, all of them must match for the violation to be ignored. Empty ignore rules are rejected because they are too broad.

## CI Example

```yaml
name: go-importflow

on:
  pull_request:
  push:
    branches: [main]

jobs:
  go-importflow:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go run ./cmd/go-importflow -config go-importflow.json
```

## Exit Codes

- `0`: no violations
- `1`: dependency boundary violations found
- `2`: configuration or analysis error

## License

MIT
