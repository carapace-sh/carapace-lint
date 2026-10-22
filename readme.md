# carapace-lint

[golangci-lint](https://golangci-lint.run) module plugin that lints
[carapace](https://github.com/carapace-sh/carapace) command definitions.

## Checks

| Settings key         | Analyzer           | Description                                                            |
|----------------------|--------------------|------------------------------------------------------------------------|
| `flags-order`        | `FlagsOrder`       | flags defined with `cmd.Flags().<Type>(...)` have to be sorted         |
| `flag-actions-order` | `FlagActionsOrder` | entries of `FlagCompletion(carapace.ActionMap{...})` have to be sorted |

Both checks group consecutive definitions, so blank lines (and comments) separate
independent blocks and single definitions are ignored. Multi-line definitions are supported.

Suggested fixes are provided for both checks and are applied by `golangci-lint run --fix`.
The reordered block is formatted in place, so the result stays compatible with `gofmt`.
A fix is not offered when it can't be applied safely, e.g. when a definition shares a line
with other code, is preceded by a comment, or when formatting would affect other lines.
The issue is reported nonetheless.

Only flags registered via `cmd.Flags()` are checked, `PersistentFlags()` is ignored
(same as the linter this replaces).

## Usage

Module plugins have to be compiled into a custom golangci-lint binary:

```shell
golangci-lint custom
./custom-gcl run ./...
```

`.custom-gcl.yml`:

```yaml
version: v2.15.0
plugins:
  # released version (`latest` works as well)
  - module: github.com/carapace-sh/carapace-lint
    import: github.com/carapace-sh/carapace-lint
    version: v0.1.0

  # or a local checkout
  # - module: github.com/carapace-sh/carapace-lint
  #   path: ../carapace-lint
```

`.golangci.yml`:

```yaml
version: "2"

linters:
  default: none
  enable:
    - carapace
  settings:
    custom:
      carapace:
        type: module
        description: check alphabetical order of carapace command definitions
        original-url: https://github.com/carapace-sh/carapace-lint
        settings:
          flags-order: true
          flag-actions-order: true
```

The linter name (`carapace` above) has to match the registered plugin name.
`carapace-lint` is registered as an alias if you prefer that name.

Issues are reported per analyzer, e.g.:

```text
main.go:26:2: FlagsOrder: flag "alpha" should be sorted before "beta" (carapace)
main.go:30:3: FlagActionsOrder: flag completion "alpha" should be sorted before "zeta" (carapace)
```

Files can be excluded with the usual golangci-lint mechanisms, e.g. to skip tests:

```yaml
linters:
  exclusions:
    rules:
      - path: (.+)_test\.go
        linters:
          - carapace
```

## Settings

| Key                  | Type | Default | Description                                  |
|----------------------|------|---------|----------------------------------------------|
| `flags-order`        | bool | `true`  | check alphabetical order of flag definitions |
| `flag-actions-order` | bool | `true`  | check alphabetical order of flag completions |

Unknown keys are rejected.

## CI

SARIF reports, code scanning and pull request annotations are provided by
golangci-lint itself and don't need to be handled by the plugin:

```yaml
output:
  formats:
    sarif:
      path: golangci-lint.sarif
```

```shell
./custom-gcl run --fix ./...
```

```yaml
- uses: github/codeql-action/upload-sarif@v4
  with:
    sarif_file: golangci-lint.sarif
```

## Development

```shell
go test ./...
```

The analyzers are tested with
[analysistest](https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest),
including the suggested fixes (`testdata/src/*/*.go.golden`).
