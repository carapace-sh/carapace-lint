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
		{name: "nil", settings: nil, want: 2},
		{name: "empty", settings: map[string]any{}, want: 2},
		{name: "flags order disabled", settings: map[string]any{"flags-order": false}, want: 1},
		{name: "flag actions order disabled", settings: map[string]any{"flag-actions-order": false}, want: 1},
		{name: "all enabled", settings: map[string]any{"flags-order": true, "flag-actions-order": true}, want: 2},
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

	p, err := New(map[string]any{"flags-order": false, "flag-actions-order": false})
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
