package carapacelint

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var completionValuesAnalyzer = &analysis.Analyzer{
	Name: "CompletionValues",
	Doc:  "check that completion values are consistent and unique",
	Run:  runCompletionValues,
}

// valueAction describes a carapace action taking completion values.
type valueAction struct {
	stride int    // number of arguments per entry
	shape  string // expected arguments, empty if values are not paired
}

// valueActions are the carapace actions taking completion values.
var valueActions = map[string]valueAction{
	"ActionStyles":                {stride: 1},
	"ActionValues":                {stride: 1},
	"ActionValuesDescribed":       {stride: 2, shape: "value/description pairs"},
	"ActionStyledValues":          {stride: 2, shape: "value/style pairs"},
	"ActionStyledValuesDescribed": {stride: 3, shape: "value/description/style triples"},
}

func runCompletionValues(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			name, ok := carapaceAction(call.Fun)
			if !ok {
				return true
			}

			if action, ok := valueActions[name]; ok {
				checkCompletionValues(pass, call, name, action)
			}

			return true
		})
	}

	return nil, nil
}

func checkCompletionValues(pass *analysis.Pass, call *ast.CallExpr, name string, action valueAction) {
	// the number of values is unknown for spread and multi-value arguments
	if action.shape != "" && !call.Ellipsis.IsValid() && !returnsMultipleValues(call.Args) && len(call.Args)%action.stride != 0 {
		pass.Report(analysis.Diagnostic{
			Pos:     call.Pos(),
			Message: fmt.Sprintf("carapace.%s expects %s, got %d arguments", name, action.shape, len(call.Args)),
		})
	}

	checkDuplicateValues(pass, call, action.stride)
}

// checkDuplicateValues reports entries that are defined multiple times within the same action.
// The same value with a different description or style is allowed.
func checkDuplicateValues(pass *analysis.Pass, call *ast.CallExpr, stride int) {
	seen := make(map[string]bool, len(call.Args)/stride)

	for i := 0; i+stride <= len(call.Args); i += stride {
		entry, ok := stringEntry(call.Args[i : i+stride])
		if !ok {
			continue // entry values are not defined statically
		}

		if entry[0] == "" {
			continue // empty values are ignored by carapace
		}

		key := fmt.Sprintf("%q", entry)
		if seen[key] {
			pass.Report(analysis.Diagnostic{
				Pos:     call.Args[i].Pos(),
				Message: fmt.Sprintf("duplicate completion entry %q", entry[0]),
			})

			continue
		}

		seen[key] = true
	}
}

// stringEntry returns the unquoted values of a completion entry.
func stringEntry(args []ast.Expr) ([]string, bool) {
	entry := make([]string, 0, len(args))

	for _, arg := range args {
		literal, ok := arg.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return nil, false
		}

		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return nil, false
		}

		entry = append(entry, value)
	}

	return entry, true
}

// carapaceAction returns the name of the carapace action called by fun.
func carapaceAction(fun ast.Expr) (string, bool) {
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "carapace" {
		return "", false
	}

	return selector.Sel.Name, true
}

// returnsMultipleValues reports whether args is a single call that may return multiple values.
func returnsMultipleValues(args []ast.Expr) bool {
	if len(args) != 1 {
		return false
	}

	_, ok := args[0].(*ast.CallExpr)

	return ok
}
