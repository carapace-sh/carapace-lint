package carapacelint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestUnitOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      string
		reorderable bool
	}{
		{
			name:        "definition",
			source:      "package p\n\nfunc f() {\n\tx.Flags().String(\"a\", \"\", \"\")\n}\n",
			reorderable: true,
		},
		{
			name:        "trailing comment",
			source:      "package p\n\nfunc f() {\n\tx.Flags().String(\"a\", \"\", \"\") // a\n}\n",
			reorderable: true,
		},
		{
			name:        "trailing block comment",
			source:      "package p\n\nfunc f() {\n\tx.Flags().String(\"a\", \"\", \"\") /* a */\n}\n",
			reorderable: true,
		},
		{
			name:        "leading comment",
			source:      "package p\n\nfunc f() {\n\t// a is deprecated\n\tx.Flags().String(\"a\", \"\", \"\")\n}\n",
			reorderable: false,
		},
		{
			name:        "leading block comment",
			source:      "package p\n\nfunc f() {\n\t/* a is deprecated */\n\tx.Flags().String(\"a\", \"\", \"\")\n}\n",
			reorderable: false,
		},
		{
			name:        "multiline block comment",
			source:      "package p\n\nfunc f() {\n\tx.Flags().String(\"a\", \"\", \"\") /* a\n\tb */\n}\n",
			reorderable: false,
		},
		{
			name:        "shared line",
			source:      "package p\n\nfunc f() {\n\ty := 1; x.Flags().String(\"a\", \"\", \"\")\n}\n",
			reorderable: false,
		},
		{
			name:        "following code",
			source:      "package p\n\nfunc f() {\n\tx.Flags().String(\"a\", \"\", \"\"); y := 1\n}\n",
			reorderable: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", test.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			var call *ast.CallExpr
			ast.Inspect(file, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok && call == nil {
					call = c
				}

				return true
			})

			if call == nil {
				t.Fatal("no call expression found")
			}

			_, ok := unitOf(fset.File(file.Pos()), []byte(test.source), "a", call)
			if ok != test.reorderable {
				t.Errorf("got reorderable %t, want %t", ok, test.reorderable)
			}
		})
	}
}
