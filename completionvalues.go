package carapacelint

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

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

	duplicates := checkDuplicateValues(pass, file, call, action.stride)
	checkSortedValues(pass, file, call, action, duplicates)
}

// checkSortedValues reports values that are not sorted alphabetically.
// Only entries that are defined statically next to each other are compared,
// as the values of everything else are unknown.
func checkSortedValues(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, action valueAction, duplicates []duplicate) {
	// a carapace:curated comment marks the list as intentionally ordered
	if curatedCall(pass, file, call) {
		return
	}

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
	duplicateIndexes := make(map[int]bool, len(duplicates))

	for _, dup := range duplicates {
		duplicateIndexes[dup.index] = true
	}

	for i := 0; i+stride <= len(call.Args); i += stride {
		entry := completionEntry{
			index: i,
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

		reportSortedEntries(pass, file, call, entries[i:next], duplicateIndexes)
		i = next + 1
	}
}

func reportSortedEntries(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, entries []completionEntry, duplicateIndexes map[int]bool) {
	less := runOrder(entries)
	if less == nil {
		less = func(a, b string) bool { return a < b }
	}

	var diagnostics []analysis.Diagnostic

	for i := 1; i < len(entries); i++ {
		previous, current := entries[i-1], entries[i]
		if !less(current.value, previous.value) {
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
	hasDuplicates := false

	for _, entry := range entries {
		if duplicateIndexes[entry.index] {
			hasDuplicates = true
			break
		}
	}

	if !hasDuplicates {
		if fix := sortedEntryFix(pass, file, call, entries, less); fix != nil {
			diagnostics[0].SuggestedFixes = []analysis.SuggestedFix{*fix}
		}
	}

	for _, diagnostic := range diagnostics {
		pass.Report(diagnostic)
	}
}

// carapaceCurated marks a value list as intentionally ordered when used in a
// comment inside the call.
const carapaceCurated = "carapace:curated"

// curatedCall reports whether a comment inside the call arguments carries the
// carapace:curated marker.
func curatedCall(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	tf := pass.Fset.File(call.Pos())
	if tf == nil {
		return false
	}

	start, end := tf.Offset(call.Lparen), tf.Offset(call.Rparen)

	for _, group := range file.Comments {
		groupStart, groupEnd := tf.Offset(group.Pos()), tf.Offset(group.End())
		if groupStart >= end || groupEnd <= start {
			continue
		}

		for _, comment := range group.List {
			if strings.Contains(comment.Text, carapaceCurated) {
				return true
			}
		}
	}

	return false
}

// runOrder returns the comparator for the statically defined entries of a run:
// numeric and size-like values are ordered by magnitude, everything else
// byte-wise. A nil comparator falls back to the byte order.
func runOrder(entries []completionEntry) func(a, b string) bool {
	for _, entry := range entries {
		if _, ok := magnitude(entry.value); !ok {
			return nil
		}
	}

	return func(a, b string) bool {
		ma, _ := magnitude(a)
		mb, _ := magnitude(b)

		return ma < mb
	}
}

// sizePattern matches numeric or size-like values, e.g. 512, 2k or 2.5MiB,
// with the mantissa and suffix captured.
var sizePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([a-z]*)$`)

// sizeSteps maps the size suffixes to the power of 1024 they represent.
var sizeSteps = map[string]int{
	"": 0, "b": 0,
	"k": 1, "ki": 1, "kb": 1, "kib": 1,
	"m": 2, "mi": 2, "mb": 2, "mib": 2,
	"g": 3, "gi": 3, "gb": 3, "gib": 3,
	"t": 4, "ti": 4, "tb": 4, "tib": 4,
	"p": 5, "pi": 5, "pb": 5, "pib": 5,
	"e": 6, "ei": 6, "eb": 6, "eib": 6,
}

// magnitude returns the value of a numeric or size-like value, e.g. 1k is
// returned as 1024, or reports false for anything else.
func magnitude(value string) (float64, bool) {
	parts := sizePattern.FindStringSubmatch(strings.ToLower(value))
	if parts == nil {
		return 0, false
	}

	mantissa, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, false
	}

	step, ok := sizeSteps[parts[2]]
	if !ok {
		return 0, false
	}

	return mantissa * math.Pow(1024, float64(step)), true
}

// sortedEntryFix returns a fix that sorts the lines of statically defined entries
// by value, or nil if they can't be reordered.
func sortedEntryFix(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, entries []completionEntry, less func(a, b string) bool) *analysis.SuggestedFix {
	tf := pass.Fset.File(call.Pos())
	if tf == nil {
		return nil
	}

	content := fileContent(pass, tf)
	if content == nil {
		return nil
	}

	units := make([]unit, 0, len(entries))
	wholeLines := true

	for _, entry := range entries {
		u, ok := unitOf(tf, content, entry.value, posSpan{start: entry.start, end: entry.end})
		if !ok {
			wholeLines = false
			break
		}

		units = append(units, u)
	}

	if wholeLines {
		if fix := reorderFix(tf, content, "sort completion values", units, less); fix != nil {
			return fix
		}
	}

	return inlineSortedFix(tf, content, entries, less)
}

// inlineSortedFix returns a fix that sorts entries written next to each other,
// on one or more lines. The entry literals are spliced in sorted order while
// the separators between them (including line breaks and indentation) stay in
// place, so the line structure is preserved. Comments or other syntax in
// between abort the fix.
func inlineSortedFix(tf *token.File, content []byte, entries []completionEntry, less func(a, b string) bool) *analysis.SuggestedFix {
	for _, entry := range entries {
		if !entry.static {
			return nil
		}
	}

	start := tf.Offset(entries[0].start)
	end := tf.Offset(entries[len(entries)-1].end)

	// separators between the entries, in source order
	var separators [][]byte

	offset := start
	for i, entry := range entries {
		entryStart := tf.Offset(entry.start)
		if entryStart < offset {
			return nil
		}

		separator := content[offset:entryStart]
		for _, c := range string(separator) {
			switch c {
			case ',', ' ', '\t', '\r', '\n':
			default:
				return nil // comments or other syntax in between
			}
		}

		if i > 0 {
			separators = append(separators, separator)
		}

		offset = tf.Offset(entry.end)
	}

	sortedEntries := make([]completionEntry, len(entries))
	copy(sortedEntries, entries)
	sort.SliceStable(sortedEntries, func(i, j int) bool {
		return less(sortedEntries[i].value, sortedEntries[j].value)
	})

	var text strings.Builder
	for i, entry := range sortedEntries {
		text.Write(content[tf.Offset(entry.start):tf.Offset(entry.end)])

		if i < len(separators) {
			text.Write(separators[i])
		}
	}

	formatted, ok := formatRegion(content, start, end, []byte(text.String()))
	if !ok {
		return nil
	}

	return &analysis.SuggestedFix{
		Message: "sort completion values alphabetically",
		TextEdits: []analysis.TextEdit{{
			Pos:     tf.Pos(start),
			End:     tf.Pos(end),
			NewText: formatted,
		}},
	}
}

// checkDuplicateValues reports entries that are defined multiple times within the same action.
// The same value with a different description or style is allowed.
func checkDuplicateValues(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stride int) []duplicate {
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
		return nil
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

	return duplicates
}

// completionEntry is one entry of a value action call.
type completionEntry struct {
	value  string    // unquoted value, empty if not statically defined
	index  int       // index of the value within the call arguments
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

			// the newline in front of the next argument is taken as well
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
