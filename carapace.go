// Package carapacelint provides a golangci-lint module plugin that checks
// carapace command definitions for alphabetical ordering.
package carapacelint

import (
	"errors"

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
}

func (s Settings) flagsOrder() bool {
	return s.FlagsOrder == nil || *s.FlagsOrder
}

func (s Settings) flagActionsOrder() bool {
	return s.FlagActionsOrder == nil || *s.FlagActionsOrder
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

// BuildAnalyzers creates the analyzers of the enabled checks.
func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	var analyzers []*analysis.Analyzer

	if p.settings.flagsOrder() {
		analyzers = append(analyzers, flagsOrderAnalyzer)
	}

	if p.settings.flagActionsOrder() {
		analyzers = append(analyzers, flagActionsOrderAnalyzer)
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
