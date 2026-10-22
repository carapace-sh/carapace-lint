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
