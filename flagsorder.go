package carapacelint

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var flagsOrderAnalyzer = &analysis.Analyzer{
	Name: "FlagsOrder",
	Doc:  "check that flags are defined in alphabetical order",
	Run:  runFlagsOrder,
}

// flagSetter matches flag definitions of pflag and its carapace fork,
// e.g. `String`, `BoolP`, `Int64SliceVarS` or `StringToStringVar`.
var flagSetter = regexp.MustCompile(`^(?:Bool|BytesBase64|BytesHex|Count|Duration|Float|Func|IP|IPMask|IPNet|Int|String|Text|Time|Uint)(?:32|64|8|16|Slice|Array|Var|P|N|S|ToString|ToInt64|ToInt|Func)*$`)

type flagDefinition struct {
	name      string
	node      *ast.CallExpr
	firstLine int
	lastLine  int
}

func runFlagsOrder(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tf := pass.Fset.File(file.Pos())

		definitions := flagDefinitions(tf, file)
		if len(definitions) < 2 {
			continue
		}

		content := fileContent(pass, tf)

		for _, group := range consecutiveDefinitions(definitions) {
			reportFlagsOrder(pass, tf, content, group)
		}
	}

	return nil, nil
}

func reportFlagsOrder(pass *analysis.Pass, tf *token.File, content []byte, definitions []flagDefinition) {
	var diagnostics []analysis.Diagnostic

	for i := 1; i < len(definitions); i++ {
		previous, current := definitions[i-1], definitions[i]
		if previous.name <= current.name {
			continue
		}

		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     current.node.Pos(),
			Message: fmt.Sprintf("flag %q should be sorted before %q", current.name, previous.name),
		})
	}

	if len(diagnostics) == 0 {
		return
	}

	if fix := reorderFix(tf, content, "sort flags alphabetically", flagUnits(tf, content, definitions)); fix != nil {
		diagnostics[0].SuggestedFixes = []analysis.SuggestedFix{*fix}
	}

	for _, diagnostic := range diagnostics {
		pass.Report(diagnostic)
	}
}

// flagDefinitions returns all flag definitions of file in source order.
func flagDefinitions(tf *token.File, file *ast.File) []flagDefinition {
	var definitions []flagDefinition

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !flagSetter.MatchString(selector.Sel.Name) {
			return true
		}

		flags, ok := selector.X.(*ast.CallExpr)
		if !ok || len(flags.Args) != 0 {
			return true
		}

		if flagSet, ok := flags.Fun.(*ast.SelectorExpr); !ok || flagSet.Sel.Name != "Flags" {
			return true
		}

		name, ok := stringLiteralArg(call.Args, 0)
		if !ok {
			return true
		}

		definitions = append(definitions, flagDefinition{
			name:      name,
			node:      call,
			firstLine: tf.Line(call.Pos()),
			lastLine:  tf.Line(call.End()),
		})

		return true
	})

	return definitions
}

// consecutiveDefinitions groups definitions that are defined on consecutive lines.
func consecutiveDefinitions(definitions []flagDefinition) [][]flagDefinition {
	var groups [][]flagDefinition

	for _, definition := range definitions {
		if n := len(groups); n == 0 || groups[n-1][len(groups[n-1])-1].lastLine+1 != definition.firstLine {
			groups = append(groups, []flagDefinition{definition})
			continue
		}

		groups[len(groups)-1] = append(groups[len(groups)-1], definition)
	}

	return groups
}

// flagUnits returns the lines of definitions, or nil if they can't be reordered.
func flagUnits(tf *token.File, content []byte, definitions []flagDefinition) []unit {
	units := make([]unit, 0, len(definitions))

	for _, definition := range definitions {
		u, ok := unitOf(tf, content, definition.name, definition.node)
		if !ok {
			return nil
		}

		units = append(units, u)
	}

	return units
}

// stringLiteralArg returns the unquoted string at args[index].
func stringLiteralArg(args []ast.Expr, index int) (string, bool) {
	if index >= len(args) {
		return "", false
	}

	return stringLiteral(args[index])
}

// stringLiteral returns the unquoted string of expr.
func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}

	return value, true
}
