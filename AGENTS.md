# carapace-lint

A [golangci-lint](https://golangci-lint.run/docs/plugins/module-plugins/) **module plugin** that lints
[carapace](https://github.com/carapace-sh/carapace) command definitions. It replaces the former
`carapace-bin/cmd/carapace-lint` command: the checks run inside an existing golangci-lint run, so
configuration, SARIF reports, pull request annotations and `--fix` are reused. A standalone binary
with default settings is shipped as well (`cmd/carapace-lint`).

Used by [carapace-bin](https://github.com/carapace-sh/carapace-bin) (completers and shared actions) and
by CLIs integrating carapace.

## Commands

| Command | Notes |
|---------|-------|
| `go test ./...` | Analyzer tests (`analysistest`, including suggested fixes) |
| `gofmt -l -s .` | Format check (must produce no output) |
| `go vet ./...` | Vet |
| `golangci-lint custom` | Build `./custom-gcl` from `.custom-gcl.yml` (clones and builds golangci-lint) |
| `./custom-gcl run ./...` | Run the checks |
| `./custom-gcl cache clean` | **Required after changing the plugin** — results are cached per binary version |
| `./custom-gcl run --max-issues-per-linter=0 --max-same-issues=0 ./...` | Untruncated results (default cap is 50) |
| `go run ./cmd/carapace-lint ./...` | Standalone binary (all checks, default settings) |
| `go run ./cmd/carapace-lint -fix ./...` | Apply the suggested fixes |
| `go run ./cmd/carapace-lint help` | List analyzers and their flags |

No Makefile — CI is `.github/workflows/test.yml`, releases are `.github/workflows/release.yml`.

To test a change against a real repository, build the custom binary into a temp dir:

```sh
mkdir -p /tmp/gcl
git clone --depth 1 --branch v2.15.0 https://github.com/golangci/golangci-lint /tmp/gcl/golangci-lint
cd /tmp/gcl/golangci-lint
printf 'package main\n\nimport (\n\t_ "github.com/carapace-sh/carapace-lint"\n)\n' > cmd/golangci-lint/plugins.go
go mod edit -replace github.com/carapace-sh/carapace-lint=/path/to/carapace-lint
go mod tidy
go build -o /tmp/gcl/custom-gcl ./cmd/golangci-lint
cd /path/to/carapace-bin && /tmp/gcl/custom-gcl run ./pkg/actions/... ./completers/...
```

## Structure

```
carapace.go          plugin registration, Settings, BuildAnalyzers, Analyzers, load mode
flagsorder.go        FlagsOrder: alphabetical cmd.Flags().<Type>(...) definitions
flagactionsorder.go  FlagActionsOrder: alphabetical FlagCompletion(ActionMap{...}) entries
macrosyntax.go       MacroSyntax: public actions must be representable as macros
completionvalues.go  CompletionValues: value arity, duplicate entries and sorted values
reorder.go           shared reordering and suggested fix helpers
cmd/carapace-lint/   standalone driver (multichecker) published by goreleaser
*_test.go            plugin, analyzer, guard and fix tests
testdata/src/...     analysistest packages (GOPATH layout, `// want` comments, `.golden` files)
```

## Checks

| Analyzer | Settings key | Scope | Enforces |
|----------|--------------|-------|----------|
| `FlagsOrder` | `flags-order` | all files | consecutive `cmd.Flags()` definitions sorted alphabetically |
| `FlagActionsOrder` | `flag-actions-order` | all files | `ActionMap` entries of one `FlagCompletion` sorted alphabetically |
| `MacroSyntax` | `macro-syntax` (+ `macro-syntax-paths`) | files matching `(^|/)pkg/actions(/|$)` | `func ActionX(...) carapace.Action` with at most one parameter, a doc comment and a single unnamed result |
| `CompletionValues` | `completion-values` | all files | argument arity of `carapace.ActionStyles/Values/...`, unique and sorted entries |

Macros are discovered by scanning package documentation (`spec.ScanMacros`), which is why `MacroSyntax`
mirrors the generator textually instead of using type information.

## Adding a check

1. Add an analyzer file: exported-or-unexported `var xAnalyzer = &analysis.Analyzer{...}` (or a constructor
   when it needs settings, see `newMacroSyntaxAnalyzer`).
2. Add a `*bool` setting to `Settings` with a kebab-case `json` tag, a `true` default accessor and a doc comment.
3. Wire it in `BuildAnalyzers` and update the analyzer counts in `carapace_test.go` (`TestNew`,
   `TestNewAllDisabled`).
4. Add a testdata package and an entry in `analyzers_test.go`; use `// want` comments and
   `RunWithSuggestedFixes` + `.golden` when the check provides fixes.
5. Document it in `README.md` (checks table, settings table, example output).
6. Verify against carapace-bin with a fresh custom binary and a cleaned cache.

## Release

Pushing a `v*` tag runs `.github/workflows/release.yml`:

1. run the analyzer tests
2. `goreleaser/goreleaser-action@v7` runs `release --clean` using `.goreleaser.yml`
3. linux/windows/darwin binaries of `cmd/carapace-lint` are attached to the GitHub release,
   with the version injected via `-ldflags -X main.version={{.Version}}`

The module itself is usable without a release (`go get github.com/carapace-sh/carapace-lint@vX`),
the binaries only provide the standalone entry point. Building the plugin into golangci-lint is
always done by the consumer via `golangci-lint custom`.

## Conventions

- Messages are lowercase, one sentence, and name the offending symbol: `public action "ActionX" should ...`.
- One diagnostic per rule and position; no cascading reports inside one finding.
- Suggested fixes reorder whole lines, are formatted in place (`formatRegion`) and are dropped when
  they would change other lines. Duplicate completion entries are removed together with an adjacent
  comma instead; the deletion ranges stay disjoint by design.
- Consecutive fixes are disjoint by design: each removed entry claims its leading
  comma on the same line, then the trailing one, then any leading comma, otherwise
  only the entry itself (a leftover trailing comma stays valid Go).
- Analyzer names are valid Go identifiers (go/analysis requirement); settings keys are kebab-case.
- Syntax-only checks assume an unaliased `carapace.` qualifier — this matches how the macro generator
  and `go doc` spell types, so it is intentional rather than a limitation to fix.
- Only `cmd.Flags()` is inspected, `PersistentFlags()` is ignored (parity with the replaced linter).

## Gotchas

- **Load mode is `syntax`**: there is no type information, and `pass.Pkg` may be nil because golangci-lint
  only loads types when an enabled linter needs them. Scope checks by file path
  (`pass.Fset.Position(...).Filename`, see `macro-syntax-paths`), never by `pass.Pkg.Path()`.
- **Cached results**: a custom build keeps golangci-lint's version string, so stale results are returned
  after changing the plugin. Always `./custom-gcl cache clean`.
- **Issue caps**: the default `--max-issues-per-linter=50` truncates output, which looks like missing
  findings. Disable the caps when comparing runs.
- **Plugin name = linter name**: `register.Plugin` is called for `carapace` and the alias `carapace-lint`;
  the key in `linters.settings.custom.<name>` has to match one of them.
- **File access**: golangci-lint sets `pass.ReadFile` (restricted to the package's files),
  `analysistest` does not; `fileContent` therefore falls back to `os.ReadFile`.
- **Fixes must stay gofmt-clean**: reordering lines changes tabwriter alignment, so the reordered block is
  formatted with `go/format` and the fix is dropped when formatting would affect lines outside the block.
  A non-gofmt'ed file therefore silently loses fixes: keep testdata and verified targets gofmt-clean.
- **Duplicate completion entries compare value, description and style**: the same value with a different
  description is intentional in carapace (see `pkg/actions/fs/extension.go` in carapace-bin) and must not
  be reported.
- **Runtime values are skipped**: arity, duplicate and sorting checks only see literal arguments of
  `carapace.Action*` calls; anything computed at runtime is ignored to avoid false positives.
- **Numeric and size-like lists are ordered by magnitude**: when every value of a run parses as
  1024-based size or plain number (`512`, `2k`, `2.5MiB`, `2.80`), sorting compares magnitudes;
  one different value makes the whole run byte-wise. `reorderFix` takes the comparator, so the
  line and splice paths agree.
- **Fixes don't overlap**: a call with duplicate entries gets no sorting fix, as the removals
  touch the same lines; the sorting is offered on a subsequent run.
- **Ordering policy**: value lists are always checked alphabetically; deliberate grouping is
  expressed with `Batch` (each call is checked independently, so precedence between the calls
  stays free) and outliers are annotated with golangci-lint's `//nolint:carapace`.
- **Sorting splices entry segments**: entries (value with description and style) are sorted in place
  and the separators between them, including line breaks and indentation, stay in their sequence
  position, so mixed line layouts keep their structure. Runs containing comments can't be spliced.
- **The standalone binary has no settings**: `cmd/carapace-lint` runs `Analyzers()` (default settings)
  and only supports the driver's own flags (`-fix`, `-json`, `-<Analyzer>=false`). Anything that needs
  `Settings` (e.g. `macro-syntax-paths`) must go through the golangci-lint plugin.

## Related

- golangci-lint module plugin system: https://golangci-lint.run/docs/plugins/module-plugins/
- New linters guide: https://golangci-lint.run/docs/contributing/new-linters/
- Macro format and signature generation: https://github.com/carapace-sh/carapace-spec (`macro.go`, `scan.go`)
- carapace-bin conventions (completers, shared actions, code generation): `../carapace-bin/AGENTS.md`
