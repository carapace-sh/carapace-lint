package tools

import "carapace"

type Opts struct{}

// ActionNoArgs completes nothing.
func ActionNoArgs() carapace.Action { return carapace.Action{} }

// ActionSingle takes an option struct.
func ActionSingle(opts Opts) carapace.Action { return carapace.Action{} }

// ActionPointer takes an option struct pointer.
func ActionPointer(opts *Opts) carapace.Action { return carapace.Action{} }

// ActionVariadic takes variadic arguments.
func ActionVariadic(names ...string) carapace.Action { return carapace.Action{} }

// ActionTwo takes two parameters.
func ActionTwo(name, value string) carapace.Action { return carapace.Action{} } // want `public action "ActionTwo" should take at most one parameter`

// ActionSum takes a command and an option struct.
func ActionSum(cmd string, opts Opts) carapace.Action { // want `public action "ActionSum" should take at most one parameter`
	return carapace.Action{}
}

// ActionVariadicAndMore takes a command and variadic arguments.
func ActionVariadicAndMore(cmd string, names ...string) carapace.Action { // want `public action "ActionVariadicAndMore" should take at most one parameter`
	return carapace.Action{}
}

// ActionNoResult returns nothing.
func ActionNoResult() {} // want `public action "ActionNoResult" should return carapace.Action`

// ActionPair returns an error as well.
func ActionPair() (carapace.Action, error) { // want `public action "ActionPair" should return carapace.Action`
	return carapace.Action{}, nil
}

// ActionNamed uses a named result.
func ActionNamed() (action carapace.Action) { // want `public action "ActionNamed" should return carapace.Action`
	return carapace.Action{}
}

// ActionTwoProblems has two problems.
func ActionTwoProblems(name, value string) string { // want `public action "ActionTwoProblems" should return carapace.Action` `public action "ActionTwoProblems" should take at most one parameter`
	return name + value
}

func ActionUndocumented() carapace.Action { // want `public action "ActionUndocumented" should have a doc comment`
	return carapace.Action{}
}

// Values is missing the Action prefix.
func Values() carapace.Action { // want `exported function "Values" should be prefixed with Action`
	return carapace.Action{}
}

// helper is not a macro.
func helper(name, value string) carapace.Action { return carapace.Action{} }

// actionUnexported is not a macro.
func actionUnexported(name, value string) carapace.Action { return carapace.Action{} }

type command struct{}

// ActionMethod is not a macro.
func (command) ActionMethod(name, value string) carapace.Action { return carapace.Action{} }
