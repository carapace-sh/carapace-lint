package carapacelint

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
)

func TestRegistration(t *testing.T) {
	t.Parallel()

	for _, name := range []string{Name, NameLint} {
		newPlugin, err := register.GetPlugin(name)
		if err != nil {
			t.Fatalf("GetPlugin(%q): %v", name, err)
		}

		if _, err := newPlugin(nil); err != nil {
			t.Fatalf("New(%q): %v", name, err)
		}
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings any
		want     int
	}{
		{name: "nil", settings: nil, want: 3},
		{name: "empty", settings: map[string]any{}, want: 3},
		{name: "flags order disabled", settings: map[string]any{"flags-order": false}, want: 2},
		{name: "flag actions order disabled", settings: map[string]any{"flag-actions-order": false}, want: 2},
		{name: "macro syntax disabled", settings: map[string]any{"macro-syntax": false}, want: 2},
		{name: "all enabled", settings: map[string]any{"flags-order": true, "flag-actions-order": true, "macro-syntax": true}, want: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			p, err := New(test.settings)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			analyzers, err := p.BuildAnalyzers()
			if err != nil {
				t.Fatalf("BuildAnalyzers: %v", err)
			}

			if len(analyzers) != test.want {
				t.Errorf("got %d analyzers, want %d", len(analyzers), test.want)
			}
		})
	}
}

func TestNewAllDisabled(t *testing.T) {
	t.Parallel()

	p, err := New(map[string]any{"flags-order": false, "flag-actions-order": false, "macro-syntax": false})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := p.BuildAnalyzers(); err == nil {
		t.Fatal("expected error when all checks are disabled")
	}
}

func TestNewUnknownSetting(t *testing.T) {
	t.Parallel()

	if _, err := New(map[string]any{"unknown": true}); err == nil {
		t.Fatal("expected error for unknown setting")
	}
}

func TestNewInvalidMacroSyntaxPaths(t *testing.T) {
	t.Parallel()

	p, err := New(map[string]any{"macro-syntax-paths": []any{"("}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := p.BuildAnalyzers(); err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestMacroSyntaxPaths(t *testing.T) {
	t.Parallel()

	paths, err := Settings{}.macroSyntaxPaths()
	if err != nil {
		t.Fatalf("macroSyntaxPaths: %v", err)
	}

	tests := []struct {
		filename string
		want     bool
	}{
		{filename: "/home/user/carapace-bin/pkg/actions/tools/git/ref.go", want: true},
		{filename: "/home/user/carapace-bin/pkg/actions/actions.go", want: true},
		{filename: "/home/user/carapace-bin/completers/common/git_completer/cmd/action/ref.go", want: false},
		{filename: "/home/user/carapace-bin/pkg/actionable/actionable.go", want: false},
	}

	for _, test := range tests {
		got := false
		for _, path := range paths {
			if path.MatchString(test.filename) {
				got = true
			}
		}

		if got != test.want {
			t.Errorf("%q matched %t, want %t", test.filename, got, test.want)
		}
	}
}

func TestGetLoadMode(t *testing.T) {
	t.Parallel()

	p, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if loadMode := p.GetLoadMode(); loadMode != register.LoadModeSyntax {
		t.Errorf("got load mode %q, want %q", loadMode, register.LoadModeSyntax)
	}
}
