package carapace

type Action struct{}

func ActionStyles(styles ...string) Action { return Action{} }

func ActionValues(values ...string) Action { return Action{} }

func ActionValuesDescribed(values ...string) Action { return Action{} }

func ActionStyledValues(values ...string) Action { return Action{} }

func ActionStyledValuesDescribed(values ...string) Action { return Action{} }
