package carapacelint

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// unit is a contiguous block of whole lines that can be reordered.
type unit struct {
	name  string
	start int // byte offset of the first line
	end   int // byte offset just behind the last line
}

// unitOf returns the whole lines covered by node.
// It reports false if those lines cannot be moved around safely,
// e.g. because node shares them with other code or comments.
func unitOf(tf *token.File, content []byte, name string, node ast.Node) (unit, bool) {
	line := tf.Line(node.Pos())
	start := tf.Offset(tf.LineStart(line))
	end := lineEndOffset(tf, tf.Line(node.End()))
	if start >= end || end > len(content) {
		return unit{}, false
	}

	// node has to start the line
	if len(bytes.TrimSpace(content[start:tf.Offset(node.Pos())])) != 0 {
		return unit{}, false
	}

	// comments above node are likely to describe it and would be left behind
	if commentLineAbove(tf, content, line) {
		return unit{}, false
	}

	// node has to end the line
	if !endsLine(content[tf.Offset(node.End()):end]) {
		return unit{}, false
	}

	return unit{name: name, start: start, end: end}, true
}

// commentLineAbove reports whether the line above line is a comment.
func commentLineAbove(tf *token.File, content []byte, line int) bool {
	if line < 2 {
		return false
	}

	above := bytes.TrimSpace(content[tf.Offset(tf.LineStart(line-1)):tf.Offset(tf.LineStart(line))])

	return bytes.HasPrefix(above, []byte("//")) ||
		bytes.HasPrefix(above, []byte("/*")) ||
		bytes.HasSuffix(above, []byte("*/"))
}

// endsLine reports whether only separators, whitespace or a comment follow on the line.
func endsLine(rest []byte) bool {
	rest = bytes.TrimLeft(rest, " \t,\r\n")

	switch {
	case len(rest) == 0:
		return true
	case bytes.HasPrefix(rest, []byte("//")):
		return true
	case bytes.HasPrefix(rest, []byte("/*")):
		// a block comment spanning multiple lines would be left behind
		return bytes.Contains(rest, []byte("*/"))
	default:
		return false
	}
}

// reorderFix returns a fix that sorts the given units by name using less, or
// the byte order when less is nil.
func reorderFix(tf *token.File, content []byte, message string, units []unit, less func(a, b string) bool) *analysis.SuggestedFix {
	if less == nil {
		less = func(a, b string) bool { return a < b }
	}

	if len(units) < 2 || len(content) != tf.Size() {
		return nil
	}

	for i, u := range units {
		if u.start >= u.end || u.end > len(content) {
			return nil
		}

		if i > 0 && units[i-1].end != u.start {
			return nil
		}
	}

	if content[units[len(units)-1].end-1] != '\n' {
		return nil
	}

	sorted := make([]unit, len(units))
	copy(sorted, units)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].name < sorted[j].name
	})

	var text strings.Builder
	for _, u := range sorted {
		text.Write(content[u.start:u.end])
	}

	start, end := units[0].start, units[len(units)-1].end

	// reordering changes gofmt's alignment, so the result is formatted in place
	// and the fix is dropped if that would affect anything else.
	formatted, ok := formatRegion(content, start, end, []byte(text.String()))
	if !ok {
		return nil
	}

	return &analysis.SuggestedFix{
		Message: message,
		TextEdits: []analysis.TextEdit{{
			Pos:     tf.Pos(start),
			End:     tf.Pos(end),
			NewText: formatted,
		}},
	}
}

// formatRegion replaces content[start:end] with replacement, formats the result
// and returns the formatted region. It reports false if formatting affects other lines.
func formatRegion(content []byte, start, end int, replacement []byte) ([]byte, bool) {
	edited := make([]byte, 0, len(content)-end+start+len(replacement))
	edited = append(edited, content[:start]...)
	edited = append(edited, replacement...)
	edited = append(edited, content[end:]...)

	formatted, err := format.Source(edited)
	if err != nil {
		return nil, false
	}

	prefix, suffix := content[:start], content[end:]
	if !bytes.HasPrefix(formatted, prefix) || !bytes.HasSuffix(formatted, suffix) {
		return nil, false
	}

	return formatted[len(prefix) : len(formatted)-len(suffix)], true
}

// fileContent returns the content of the file or nil if it can't be read.
func fileContent(pass *analysis.Pass, tf *token.File) []byte {
	readFile := pass.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}

	content, err := readFile(tf.Name())
	if err != nil {
		return nil
	}

	return content
}

// lineEndOffset returns the byte offset just behind line, including the newline.
func lineEndOffset(tf *token.File, line int) int {
	if line >= tf.LineCount() {
		return tf.Size()
	}

	return tf.Offset(tf.LineStart(line + 1))
}
