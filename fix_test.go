package carapacelint

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"
)

const fixHeader = `package p

type flagSet struct{}

func (f *flagSet) Bool(name string, value bool, usage string) *bool { return &value }

func (f *flagSet) String(name, value, usage string) *string { return &value }

type action string

type actionMap map[string]action

func values(values ...string) action { return action(values[0]) }

type command struct{}

func (c *command) Flags() *flagSet { return &flagSet{} }

type carapace struct{}

func Gen(cmd *command) carapace { return carapace{} }

func (c carapace) FlagCompletion(actions actionMap) {}
`

func TestFixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		analyzer *analysis.Analyzer
		body     string
		want     string // expected body after applying the fix, empty if no fix is offered
	}{
		{
			name:     "flags",
			analyzer: flagsOrderAnalyzer,
			body: `func f() {
	cmd := &command{}
	cmd.Flags().Bool("b", false, "") // b
	cmd.Flags().Bool("a", false, "") // a
}
`,
			want: `func f() {
	cmd := &command{}
	cmd.Flags().Bool("a", false, "") // a
	cmd.Flags().Bool("b", false, "") // b
}
`,
		},
		{
			name:     "flags realign trailing comments",
			analyzer: flagsOrderAnalyzer,
			body: `func f() {
	cmd := &command{}
	cmd.Flags().String("c", "", "") // c
	cmd.Flags().String("b", "", "") // b
	cmd.Flags().String("aaaaaaaaaaa", "", "") // a
}
`,
			want: `func f() {
	cmd := &command{}
	cmd.Flags().String("aaaaaaaaaaa", "", "") // a
	cmd.Flags().String("b", "", "")           // b
	cmd.Flags().String("c", "", "")           // c
}
`,
		},
		{
			name:     "flag completions",
			analyzer: flagActionsOrderAnalyzer,
			body: `func f() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"b": values("b1", "b2"),
		"a": values("a1"),
	})
}
`,
			want: `func f() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"a": values("a1"),
		"b": values("b1", "b2"),
	})
}
`,
		},
		{
			name:     "flag completions realign multiline values",
			analyzer: flagActionsOrderAnalyzer,
			body: `func f() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"bbbb": values(
			"1",
		),
		"c":  "1",
		"aa": "2",
	})
}
`,
			want: `func f() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"aa": "2",
		"bbbb": values(
			"1",
		),
		"c": "1",
	})
}
`,
		},
		{
			name:     "completion values sorted",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues(
		"b",
		"a",
	)
}
`,
			want: `func f() {
	carapace.ActionValues(
		"a",
		"b",
	)
}
`,
		},
		{
			name:     "completion values sorted inline",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues("b", "a")
}
`,
			want: `func f() {
	carapace.ActionValues("a", "b")
}
`,
		},
		{
			name:     "completion values sorted inline triple",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues("b", "a", "c")
}
`,
			want: `func f() {
	carapace.ActionValues("a", "b", "c")
}
`,
		},
		{
			name:     "completion values sorted in literals with commas",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues("b,a", "ab", "b")
}
`,
			want: `func f() {
	carapace.ActionValues("ab", "b", "b,a")
}
`,
		},
		{
			name:     "completion values sorted mixed lines",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValuesDescribed(
		"c", "description c",
		"b", "description b", "a", "description a",
	)
}
`,
			want: `func f() {
	carapace.ActionValuesDescribed(
		"a", "description a",
		"b", "description b", "c", "description c",
	)
}
`,
		},
		{
			name:     "completion values sorted with blank line between",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues(
		"b",

		"a",
	)
}
`,
			want: `func f() {
	carapace.ActionValues(
		"a",

		"b",
	)
}
`,
		},
		{
			name:     "completion values sorted sharing a line",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.Batch(carapace.ActionDirectories(), carapace.ActionValues("b", "a"))
}
`,
			want: `func f() {
	carapace.Batch(carapace.ActionDirectories(), carapace.ActionValues("a", "b"))
}
`,
		},
		{
			name:     "completion values sorted chained",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues("b", "a").Tag("x")
}
`,
			want: `func f() {
	carapace.ActionValues("a", "b").Tag("x")
}
`,
		},
		{
			name:     "completion values sorted in two runs",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.Batch(
		carapace.ActionValues("b", "a"),
		carapace.ActionDirectories(),
		carapace.ActionValues("c", "d"),
	)
}
`,
			want: `func f() {
	carapace.Batch(
		carapace.ActionValues("a", "b"),
		carapace.ActionDirectories(),
		carapace.ActionValues("c", "d"),
	)
}
`,
		},
		{
			name:     "completion values with trailing comment rides along",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues(
		"b", // b
		"a",
	)
}
`,
			want: `func f() {
	carapace.ActionValues(
		"a",
		"b", // b
	)
}
`,
		},
		{
			name:     "completion values with own comment line between",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues(
		"b",
		// a
		"a",
	)
}
`,
		},
		{
			name:     "completion values unformatted file",
			analyzer: completionValuesAnalyzer,
			body: `func f()  {
	carapace.ActionValues("b", "a")
}
`,
		},
		{
			name:     "completion values with comment between",
			analyzer: completionValuesAnalyzer,
			body: `func f() {
	carapace.ActionValues("b" /* x */, "a")
}
`,
		},
		{
			name:     "flags commented",
			analyzer: flagsOrderAnalyzer,
			body: `func f() {
	cmd := &command{}
	// a is deprecated
	cmd.Flags().Bool("b", false, "")
	cmd.Flags().Bool("a", false, "")
}
`,
		},
		{
			name:     "flags unformatted",
			analyzer: flagsOrderAnalyzer,
			body: `func f()  {
	cmd := &command{}
	cmd.Flags().Bool("b", false, "")
	cmd.Flags().Bool("a", false, "")
}
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := []byte(fixHeader + test.body)
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "p.go", source, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			tf := fset.File(file.Pos())
			var fixes []analysis.SuggestedFix

			pass := &analysis.Pass{
				Fset:  fset,
				Files: []*ast.File{file},
				ReadFile: func(string) ([]byte, error) {
					return source, nil
				},
				Report: func(diagnostic analysis.Diagnostic) {
					fixes = append(fixes, diagnostic.SuggestedFixes...)
				},
			}

			if _, err := test.analyzer.Run(pass); err != nil {
				t.Fatalf("run: %v", err)
			}

			if test.want == "" {
				if len(fixes) != 0 {
					t.Fatalf("got %d fixes, want none", len(fixes))
				}

				return
			}

			if len(fixes) != 1 {
				t.Fatalf("got %d fixes, want 1", len(fixes))
			}

			fixed := applyEdits(t, source, tf, fixes[0].TextEdits)

			if want := fixHeader + test.want; string(fixed) != want {
				t.Errorf("unexpected fix:\n-- got --\n%s\n-- want --\n%s", fixed, want)
			}

			formatted, err := format.Source(fixed)
			if err != nil {
				t.Fatalf("format: %v", err)
			}

			if !bytes.Equal(formatted, fixed) {
				t.Errorf("fixed source is not formatted:\n-- got --\n%s\n-- formatted --\n%s", fixed, formatted)
			}
		})
	}
}

func applyEdits(t *testing.T, content []byte, tf *token.File, edits []analysis.TextEdit) []byte {
	t.Helper()

	if len(edits) != 1 {
		t.Fatalf("got %d edits, want 1", len(edits))
	}

	start, end := tf.Offset(edits[0].Pos), tf.Offset(edits[0].End)

	fixed := make([]byte, 0, len(content)-end+start+len(edits[0].NewText))
	fixed = append(fixed, content[:start]...)
	fixed = append(fixed, edits[0].NewText...)
	fixed = append(fixed, content[end:]...)

	return fixed
}
