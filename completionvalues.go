package carapacelint

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var completionValuesAnalyzer = &analysis.Analyzer{
	Name: "CompletionValues",
	Doc:  "check that completion values are consistent, unique and sorted",
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
				checkCompletionValues(pass, file, call, name, action)
			}

			return true
		})
	}

	return nil, nil
}

func checkCompletionValues(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, name string, action valueAction) {
	// the number of values is unknown for spread and multi-value arguments
	if action.shape != "" && !call.Ellipsis.IsValid() && !returnsMultipleValues(call.Args) && len(call.Args)%action.stride != 0 {
		pass.Report(analysis.Diagnostic{
			Pos:     call.Pos(),
			Message: fmt.Sprintf("carapace.%s expects %s, got %d arguments", name, action.shape, len(call.Args)),
		})
	}

	duplicateCount := checkDuplicateValues(pass, file, call, action.stride)
	checkSortedValues(pass, file, call, action, duplicateCount > 0)
}

// checkSortedValues reports values that are not sorted alphabetically.
// Only entries that are defined statically next to each other are compared,
// as the values of everything else are unknown.
func checkSortedValues(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, action valueAction, hasDuplicates bool) {
	// the sequence of entries is unknown for spread and multi-value arguments
	if call.Ellipsis.IsValid() || returnsMultipleValues(call.Args) {
		return
	}

	// entry boundaries are broken, the arity is reported separately
	if action.shape != "" && len(call.Args)%action.stride != 0 {
		return
	}

	stride := action.stride
	entries := make([]completionEntry, 0, len(call.Args)/stride)

	for i := 0; i+stride <= len(call.Args); i += stride {
		entry := completionEntry{
			start: call.Args[i].Pos(),
			end:   call.Args[i+stride-1].End(),
		}

		if values, ok := stringEntry(call.Args[i : i+stride]); ok && values[0] != "" { // empty values are ignored by carapace
			entry.value = values[0]
			entry.static = true
		}

		entries = append(entries, entry)
	}

	// statically defined entries form blocks that can be compared and reordered
	for i := 0; i < len(entries); {
		next := i
		for next < len(entries) && entries[next].static {
			next++
		}

		reportSortedEntries(pass, file, call, entries[i:next], hasDuplicates)
		i = next + 1
	}
}

func reportSortedEntries(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, entries []completionEntry, hasDuplicates bool) {
	var diagnostics []analysis.Diagnostic

	for i := 1; i < len(entries); i++ {
		previous, current := entries[i-1], entries[i]
		if previous.value <= current.value {
			continue
		}

		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     current.start,
			Message: fmt.Sprintf("completion value %q should be sorted before %q", current.value, previous.value),
		})
	}

	if len(diagnostics) == 0 {
		return
	}

	// duplicate entries are removed by another fix that touches the same lines
	if !hasDuplicates {
		if fix := sortedEntryFix(pass, file, call, entries); fix != nil {
			diagnostics[0].SuggestedFixes = []analysis.SuggestedFix{*fix}
		}
	}

	for _, diagnostic := range diagnostics {
		pass.Report(diagnostic)
	}
}

// sortedEntryFix returns a fix that sorts the lines of statically defined entries
// by value, or nil if they can't be reordered.
func sortedEntryFix(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, entries []completionEntry) *analysis.SuggestedFix {
	tf := pass.Fset.File(call.Pos())
	if tf == nil {
		return nil
	}

	content := fileContent(pass, tf)
	if content == nil {
		return nil
	}

	units := make([]unit, 0, len(entries))

	for _, entry := range entries {
		u, ok := unitOf(tf, content, entry.value, posSpan{start: entry.start, end: entry.end})
		if !ok {
			return nil
		}

		units = append(units, u)
	}

	return reorderFix(tf, content, "sort completion values alphabetically", units)
}

// checkDuplicateValues reports entries that are defined multiple times within the same action.
// The same value with a different description or style is allowed.
func checkDuplicateValues(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stride int) (duplicateCount int) {
	seen := make(map[string]bool, len(call.Args)/stride)
	var duplicates []duplicate

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
			duplicates = append(duplicates, duplicate{index: i, value: entry[0]})
			continue
		}

		seen[key] = true
	}

	if len(duplicates) == 0 {
		return 0
	}

	fixes := duplicateEntryFixes(pass, file, call, stride, duplicates)

	for _, dup := range duplicates {
		diagnostic := analysis.Diagnostic{
			Pos:     call.Args[dup.index].Pos(),
			Message: fmt.Sprintf("duplicate completion entry %q", dup.value),
		}

		if fix, ok := fixes[dup.index]; ok {
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{*fix}
		}

		pass.Report(diagnostic)
	}

	return len(duplicates)
}

// completionEntry is one entry of a value action call.
type completionEntry struct {
	value  string    // unquoted value, empty if not statically defined
	start  token.Pos // position of the value
	end    token.Pos // end of the entry
	static bool      // whether the value is defined statically
}

// posSpan spans token positions and adapts them to ast.Node for reordering.
type posSpan struct {
	start token.Pos
	end   token.Pos
}

func (s posSpan) Pos() token.Pos { return s.start }

func (s posSpan) End() token.Pos { return s.end }

// duplicate is an entry that is defined multiple times within the same call.
type duplicate struct {
	index int
	value string
}

// duplicateEntryFixes returns a deletion for every duplicated entry of the call.
// The deletions can be applied together: each removed entry owns one adjacent
// comma and falls back to the entry itself when those are already in use.
func duplicateEntryFixes(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stride int, duplicates []duplicate) map[int]*analysis.SuggestedFix {
	tf := pass.Fset.File(call.Pos())
	if tf == nil {
		return nil
	}

	content := fileContent(pass, tf)
	if content == nil {
		return nil
	}

	fixes := make(map[int]*analysis.SuggestedFix, len(duplicates))
	prevEnd := 0

	for _, dup := range duplicates {
		start := tf.Offset(call.Args[dup.index].Pos())
		end := tf.Offset(call.Args[dup.index+stride-1].End())
		if start < prevEnd || start >= end || end > len(content) {
			return nil
		}

		if comma, ok := leadingComma(content, start); ok &&
			comma > prevEnd && !commentOverlaps(file, tf, comma, end) {
			start = comma
		} else if trailing, ok := trailingSeparator(content, end); ok &&
			trailing > prevEnd {
			end = trailing
		} else if comma, ok := separatorBefore(content, start); ok &&
			comma > prevEnd && !commentOverlaps(file, tf, comma, end) {
			start = comma
		}

		prevEnd = end

		fixes[dup.index] = &analysis.SuggestedFix{
			Message: "remove duplicate completion entry",
			TextEdits: []analysis.TextEdit{{
				Pos: tf.Pos(start),
				End: tf.Pos(end),
			}},
		}
	}

	return fixes
}

// leadingComma returns the position of a comma directly in front of the entry,
// on the same line.
func leadingComma(content []byte, start int) (int, bool) {
	i := start - 1
	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\r') {
		i--
	}

	if i >= 0 && content[i] == ',' {
		return i, true
	}

	return 0, false
}

// trailingSeparator returns the offset behind an entry where the deletion ends
// when its trailing comma is taken: the comma plus following whitespace, an
// inline comment and the indentation of the next argument. The newline in front
// of a closing parenthesis is kept.
func trailingSeparator(content []byte, end int) (int, bool) {
	i := end
	for i < len(content) && (content[i] == ' ' || content[i] == '\t' || content[i] == '\r') {
		i++
	}

	if i >= len(content) || content[i] != ',' {
		return 0, false
	}

	i++

	for i < len(content) {
		switch {
		case content[i] == ' ' || content[i] == '\t' || content[i] == '\r':
			i++
		case content[i] == '/' && i+1 < len(content) && content[i+1] == '/':
			for i < len(content) && content[i] != '\n' {
				i++
			}

			return i, true
		case content[i] == '/' && i+1 < len(content) && content[i+1] == '*':
			to := bytes.Index(content[i:], []byte("*/"))
			if to < 0 || bytes.IndexByte(content[i:i+to], '\n') >= 0 {
				return i, true
			}

			i += to + 2
		case content[i] == '\n':
			for j := i + 1; j < len(content); j++ {
				if content[j] != ' ' && content[j] != '\t' && content[j] != '\r' {
					if content[j] == ')' {
						return i, true
					}

					return j, true
				}
			}

			return i, true
		default:
			return i, true
		}
	}

	return i, true
}

// separatorBefore returns the position of the comma in front of an entry,
// even across a line break.
func separatorBefore(content []byte, start int) (int, bool) {
	i := start - 1
	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\r' || content[i] == '\n') {
		i--
	}

	if i >= 0 && content[i] == ',' {
		return i, true
	}

	return 0, false
}

// commentOverlaps reports whether a comment group intersects the byte range.
func commentOverlaps(file *ast.File, tf *token.File, start, end int) bool {
	for _, group := range file.Comments {
		if tf.Offset(group.Pos()) < end && start < tf.Offset(group.End()) {
			return true
		}
	}

	return false
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
