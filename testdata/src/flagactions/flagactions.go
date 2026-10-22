package flagactions

type action string

type actionMap map[string]action

func values(values ...string) action { return action(values[0]) }

type command struct{}

type carapace struct{}

func Gen(cmd *command) carapace { return carapace{} }

func (c carapace) FlagCompletion(actions actionMap) {}

func sorted() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"alpha": "a",
		"beta":  "b",
	})
}

func unsorted() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"beta":  "b",
		"alpha": "a", // want `flag completion "alpha" should be sorted before "beta"`
	})
}

func multipleBlocks() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"beta":  "b",
		"alpha": "a", // want `flag completion "alpha" should be sorted before "beta"`
	})
	Gen(cmd).FlagCompletion(actionMap{
		"gamma": "g",
		"delta": "d", // want `flag completion "delta" should be sorted before "gamma"`
	})
}

func withComment() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"delta":   "d", // some comment
		"charlie": "c", // want `flag completion "charlie" should be sorted before "delta"`
	})
}

func multilineValue() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		"beta": values(
			"b1",
			"b2",
		),
		"alpha": "a", // want `flag completion "alpha" should be sorted before "beta"`
	})
}

func singleLine() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{"gamma": "g", "alpha": "a"}) // want `flag completion "alpha" should be sorted before "gamma"`
}

func commentedEntry() {
	cmd := &command{}
	Gen(cmd).FlagCompletion(actionMap{
		// gamma is deprecated
		"gamma": "g",
		"alpha": "a", // want `flag completion "alpha" should be sorted before "gamma"`
	})
}

func dynamicKey() {
	cmd := &command{}
	key := "beta"
	Gen(cmd).FlagCompletion(actionMap{
		key:     "b",
		"alpha": "a",
	})
}
