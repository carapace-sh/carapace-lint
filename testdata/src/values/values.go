package values

import "carapace"

func values() carapace.Action {
	return carapace.ActionValues("a", "b")
}

func described() carapace.Action {
	return carapace.ActionValuesDescribed(
		"a", "description a",
		"b", "description b",
	)
}

func duplicateValues() carapace.Action {
	return carapace.ActionValues("a", "b", "a") // want `duplicate completion entry "a"`
}

func duplicateStyles() carapace.Action {
	return carapace.ActionStyles("red", "blue", "red") // want `duplicate completion entry "red"`
}

func duplicatedPairs() carapace.Action {
	return carapace.ActionValues(
		"a", "a", // want `duplicate completion entry "a"`
		"b", "b", // want `duplicate completion entry "b"`
	)
}

func duplicatedInline() carapace.Action {
	return carapace.ActionValues("a", "b", "a", "b") // want `duplicate completion entry "a"` `duplicate completion entry "b"`
}

func duplicateValuesDescribed() carapace.Action {
	return carapace.ActionValuesDescribed(
		"a", "description a",
		"b", "description b",
		"a", "description a", // want `duplicate completion entry "a"`
	)
}

func repeatedWithDifferentDescriptions() carapace.Action {
	return carapace.ActionValuesDescribed(
		"a", "description a",
		"a", "other description",
	)
}

func repeatedWithDifferentStyles() carapace.Action {
	return carapace.ActionStyledValuesDescribed(
		"a", "description a", "red",
		"a", "description a", "green",
	)
}

func duplicateStyledValuesDescribed() carapace.Action {
	return carapace.ActionStyledValuesDescribed(
		"a", "description a", "red",
		"a", "description a", "red", // want `duplicate completion entry "a"`
	)
}

func emptyValues() carapace.Action {
	return carapace.ActionValues("", "")
}

func oddValuesDescribed() carapace.Action {
	return carapace.ActionValuesDescribed("b", "description b", "a") // want `carapace.ActionValuesDescribed expects value/description pairs, got 3 arguments`
}

func oddStyledValues() carapace.Action {
	return carapace.ActionStyledValues("a", "red", "b") // want `carapace.ActionStyledValues expects value/style pairs, got 3 arguments`
}

func oddStyledValuesDescribed() carapace.Action {
	return carapace.ActionStyledValuesDescribed("a", "description a", "red", "b") // want `carapace.ActionStyledValuesDescribed expects value/description/style triples, got 4 arguments`
}

func spread(values ...string) carapace.Action {
	return carapace.ActionValuesDescribed(values...)
}

func multipleValues() carapace.Action {
	return carapace.ActionValuesDescribed(pair())
}

func pair() (string, string) {
	return "a", "b"
}

type other struct{}

func (other) ActionValuesDescribed(values ...string) carapace.Action { return carapace.Action{} }

func notCarapace(o other) carapace.Action {
	return o.ActionValuesDescribed("a", "b", "c")
}

func partiallyDynamic(a string) carapace.Action {
	return carapace.ActionValuesDescribed(
		"a", "description a",
		a, "description a",
		"a", "description a", // want `duplicate completion entry "a"`
	)
}

func runtimeValues(a, b string) carapace.Action {
	return carapace.ActionValuesDescribed(a, b)
}
