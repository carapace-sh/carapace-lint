package tools

import "carapace"

type Opts struct{}

func ActionNoArgs() carapace.Action { return carapace.Action{} }

func ActionSingle(opts Opts) carapace.Action { return carapace.Action{} }

func ActionPointer(opts *Opts) carapace.Action { return carapace.Action{} }

func ActionVariadic(names ...string) carapace.Action { return carapace.Action{} }

func ActionTwo(name, value string) carapace.Action { return carapace.Action{} } // want `public action "ActionTwo" should take at most one parameter`

func ActionSum(cmd string, opts Opts) carapace.Action { // want `public action "ActionSum" should take at most one parameter`
	return carapace.Action{}
}

func ActionVariadicAndMore(cmd string, names ...string) carapace.Action { // want `public action "ActionVariadicAndMore" should take at most one parameter`
	return carapace.Action{}
}

func ActionNoResult() {} // want `public action "ActionNoResult" should return carapace.Action`

func ActionPair() (carapace.Action, error) { // want `public action "ActionPair" should return carapace.Action`
	return carapace.Action{}, nil
}

func ActionNamed() (action carapace.Action) { // want `public action "ActionNamed" should return carapace.Action`
	return carapace.Action{}
}

func ActionTwoProblems(name, value string) string { // want `public action "ActionTwoProblems" should return carapace.Action` `public action "ActionTwoProblems" should take at most one parameter`
	return name + value
}

func helper(name, value string) carapace.Action { return carapace.Action{} }

func actionUnexported(name, value string) carapace.Action { return carapace.Action{} }

type command struct{}

func (command) ActionMethod(name, value string) carapace.Action { return carapace.Action{} }
