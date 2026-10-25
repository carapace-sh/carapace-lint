// Package carapacelint provides a golangci-lint module plugin that checks
// carapace command definitions for alphabetical ordering.
package carapacelint

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

const (
	// Name is the plugin name and has to be used as linter name within `.golangci.yml`.
	Name = "carapace"
	// NameLint is an alias for Name to allow referring to the linter as `carapace-lint`.
	NameLint = "carapace-lint"
)

func init() {
	register.Plugin(Name, New)
	register.Plugin(NameLint, New)
}

// Settings are the plugin settings.
type Settings struct {
	// FlagsOrder checks that flags are defined in alphabetical order (default: true).
	FlagsOrder *bool `json:"flags-order"`
	// FlagActionsOrder checks that flag completions are defined in alphabetical order (default: true).
	FlagActionsOrder *bool `json:"flag-actions-order"`
	// MacroSyntax checks that public actions match the macro syntax (default: true).
	MacroSyntax *bool `json:"macro-syntax"`
	// MacroSyntaxPaths restricts MacroSyntax to files matching these patterns
	// (default: `["(^|/)pkg/actions(/|$)"]`).
	MacroSyntaxPaths []string `json:"macro-syntax-paths"`
	// CompletionValues checks completion values for correct pairing, order and uniqueness (default: true).
	CompletionValues *bool `json:"completion-values"`
}

func (s Settings) flagsOrder() bool {
	return s.FlagsOrder == nil || *s.FlagsOrder
}

func (s Settings) flagActionsOrder() bool {
	return s.FlagActionsOrder == nil || *s.FlagActionsOrder
}

func (s Settings) macroSyntax() bool {
	return s.MacroSyntax == nil || *s.MacroSyntax
}

func (s Settings) completionValues() bool {
	return s.CompletionValues == nil || *s.CompletionValues
}

// macroSyntaxPaths returns the patterns of files providing public actions.
func (s Settings) macroSyntaxPaths() ([]*regexp.Regexp, error) {
	patterns := s.MacroSyntaxPaths
	if len(patterns) == 0 {
		patterns = []string{`(^|/)pkg/actions(/|$)`}
	}

	paths := make([]*regexp.Regexp, 0, len(patterns))

	for _, pattern := range patterns {
		path, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid macro-syntax-paths pattern %q: %w", pattern, err)
		}

		paths = append(paths, path)
	}

	return paths, nil
}

type plugin struct {
	settings Settings
}

// New creates the plugin.
func New(rawSettings any) (register.LinterPlugin, error) {
	settings, err := register.DecodeSettings[Settings](rawSettings)
	if err != nil {
		return nil, err
	}

	return &plugin{settings: settings}, nil
}

// Analyzers returns the analyzers of all checks with default settings.
func Analyzers() ([]*analysis.Analyzer, error) {
	p, err := New(nil)
	if err != nil {
		return nil, err
	}

	return p.BuildAnalyzers()
}

// BuildAnalyzers creates the analyzers of the enabled checks.
func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	var analyzers []*analysis.Analyzer

	if p.settings.flagsOrder() {
		analyzers = append(analyzers, flagsOrderAnalyzer)
	}

	if p.settings.flagActionsOrder() {
		analyzers = append(analyzers, flagActionsOrderAnalyzer)
	}

	if p.settings.macroSyntax() {
		paths, err := p.settings.macroSyntaxPaths()
		if err != nil {
			return nil, err
		}

		analyzers = append(analyzers, newMacroSyntaxAnalyzer(paths))
	}

	if p.settings.completionValues() {
		analyzers = append(analyzers, completionValuesAnalyzer)
	}

	if len(analyzers) == 0 {
		return nil, errors.New("all checks are disabled")
	}

	return analyzers, nil
}

// GetLoadMode returns the load mode required by the analyzers.
func (p *plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}
