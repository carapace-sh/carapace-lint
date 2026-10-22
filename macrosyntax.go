package carapacelint

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// newMacroSyntaxAnalyzer creates an analyzer checking public actions of the given paths.
func newMacroSyntaxAnalyzer(paths []*regexp.Regexp) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "MacroSyntax",
		Doc:  "check that public actions match the macro syntax",
		Run: func(pass *analysis.Pass) (any, error) {
			return runMacroSyntax(pass, paths)
		},
	}
}

func runMacroSyntax(pass *analysis.Pass, paths []*regexp.Regexp) (any, error) {
	for _, file := range pass.Files {
		if !isActionFile(pass, file, paths) {
			continue
		}

		for _, decl := range file.Decls {
			if function := macroFunction(decl); function != nil {
				checkMacroSyntax(pass, function)
			}
		}
	}

	return nil, nil
}

// isActionFile reports whether file is scanned for macros.
func isActionFile(pass *analysis.Pass, file *ast.File, paths []*regexp.Regexp) bool {
	filename := pass.Fset.Position(file.Pos()).Filename
	if strings.HasSuffix(filename, "_test.go") {
		return false
	}

	for _, path := range paths {
		if path.MatchString(filename) {
			return true
		}
	}

	return false
}

// macroFunction returns decl if it is an exported function that is or should be a macro.
// Macros are discovered by scanning the documentation for `func ActionX(...) carapace.Action`.
func macroFunction(decl ast.Decl) *ast.FuncDecl {
	function, ok := decl.(*ast.FuncDecl)
	if !ok || function.Recv != nil || !function.Name.IsExported() {
		return nil
	}

	if !strings.HasPrefix(function.Name.Name, "Action") && !returnsAction(function.Type.Results) {
		return nil
	}

	return function
}

func checkMacroSyntax(pass *analysis.Pass, function *ast.FuncDecl) {
	name := function.Name.Name
	action := strings.HasPrefix(name, "Action")

	switch {
	case action && !returnsAction(function.Type.Results):
		pass.Report(analysis.Diagnostic{
			Pos:     function.Name.Pos(),
			Message: fmt.Sprintf("public action %q should return carapace.Action to match the macro syntax", name),
		})

	case action && function.Doc == nil:
		pass.Report(analysis.Diagnostic{
			Pos:     function.Name.Pos(),
			Message: fmt.Sprintf("public action %q should have a doc comment to describe the macro", name),
		})

	case !action:
		pass.Report(analysis.Diagnostic{
			Pos:     function.Name.Pos(),
			Message: fmt.Sprintf("exported function %q should be prefixed with Action to be available as macro", name),
		})
	}

	if action && parameters(function.Type.Params) > 1 {
		pass.Report(analysis.Diagnostic{
			Pos:     function.Name.Pos(),
			Message: fmt.Sprintf("public action %q should take at most one parameter to match the macro syntax", name),
		})
	}
}

// returnsAction reports whether results is a single unnamed carapace.Action.
func returnsAction(results *ast.FieldList) bool {
	if results == nil || len(results.List) != 1 {
		return false
	}

	result := results.List[0]
	if len(result.Names) != 0 {
		return false
	}

	selector, ok := result.Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Action" {
		return false
	}

	pkg, ok := selector.X.(*ast.Ident)

	return ok && pkg.Name == "carapace"
}

// parameters returns the number of parameters of params.
func parameters(params *ast.FieldList) int {
	count := 0

	for _, field := range params.List {
		count += max(1, len(field.Names))
	}

	return count
}
