package resource

// CommandResult is a wrapper around both the input and output attributes that are relavent for updates
type CommandResult struct {
	Output string
}

// NewCommandResult is the constructor for CommandResult
func NewCommandResult() *CommandResult {
	return &CommandResult{}
}
