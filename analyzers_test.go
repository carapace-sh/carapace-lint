package carapacelint

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestFlagsOrder(t *testing.T) {
	t.Parallel()

	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), flagsOrderAnalyzer, "flags")
}

func TestFlagActionsOrder(t *testing.T) {
	t.Parallel()

	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), flagActionsOrderAnalyzer, "flagactions")
}

func TestMacroSyntax(t *testing.T) {
	t.Parallel()

	paths, err := Settings{}.macroSyntaxPaths()
	if err != nil {
		t.Fatalf("macroSyntaxPaths: %v", err)
	}

	analysistest.Run(t, analysistest.TestData(), newMacroSyntaxAnalyzer(paths), "pkg/actions/tools", "completers/action")
}

func TestCompletionValues(t *testing.T) {
	t.Parallel()

	analysistest.Run(t, analysistest.TestData(), completionValuesAnalyzer, "values")
}
