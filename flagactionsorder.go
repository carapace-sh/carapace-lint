package carapacelint

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var flagActionsOrderAnalyzer = &analysis.Analyzer{
	Name: "FlagActionsOrder",
	Doc:  "check that flag completions are defined in alphabetical order",
	Run:  runFlagActionsOrder,
}

type flagCompletion struct {
	name string
	node *ast.KeyValueExpr
}

func runFlagActionsOrder(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		completions := flagCompletions(file)
		if len(completions) == 0 {
			continue
		}

		tf := pass.Fset.File(file.Pos())
		content := fileContent(pass, tf)

		for _, entries := range completions {
			reportFlagActionsOrder(pass, tf, content, entries)
		}
	}

	return nil, nil
}

func reportFlagActionsOrder(pass *analysis.Pass, tf *token.File, content []byte, entries []flagCompletion) {
	var diagnostics []analysis.Diagnostic

	for i := 1; i < len(entries); i++ {
		previous, current := entries[i-1], entries[i]
		if previous.name <= current.name {
			continue
		}

		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     current.node.Pos(),
			Message: fmt.Sprintf("flag completion %q should be sorted before %q", current.name, previous.name),
		})
	}

	if len(diagnostics) == 0 {
		return
	}

	if fix := reorderFix(tf, content, "sort flag completions alphabetically", completionUnits(tf, content, entries)); fix != nil {
		diagnostics[0].SuggestedFixes = []analysis.SuggestedFix{*fix}
	}

	for _, diagnostic := range diagnostics {
		pass.Report(diagnostic)
	}
}

// flagCompletions returns the entries of each FlagCompletion call in file.
func flagCompletions(file *ast.File) [][]flagCompletion {
	var completions [][]flagCompletion

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "FlagCompletion" {
			return true
		}

		literal, ok := call.Args[0].(*ast.CompositeLit)
		if !ok {
			return true
		}

		if entries, ok := flagCompletionEntries(literal); ok {
			completions = append(completions, entries)
		}

		return true
	})

	return completions
}

// flagCompletionEntries returns the entries of literal if all its keys are string literals.
func flagCompletionEntries(literal *ast.CompositeLit) ([]flagCompletion, bool) {
	entries := make([]flagCompletion, 0, len(literal.Elts))

	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return nil, false
		}

		name, ok := stringLiteral(keyValue.Key)
		if !ok {
			return nil, false
		}

		entries = append(entries, flagCompletion{name: name, node: keyValue})
	}

	return entries, true
}

// completionUnits returns the lines of entries, or nil if they can't be reordered.
func completionUnits(tf *token.File, content []byte, entries []flagCompletion) []unit {
	units := make([]unit, 0, len(entries))

	for _, entry := range entries {
		u, ok := unitOf(tf, content, entry.name, entry.node)
		if !ok {
			return nil
		}

		units = append(units, u)
	}

	return units
}
