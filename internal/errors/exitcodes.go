package errors

// Exit code constants. Document new codes here before using them.
// Keep in sync with the exit code table in CLAUDE.md.
const (
	ExitSuccess        = 0
	ExitGeneralError   = 1
	ExitBadArguments   = 2
	ExitAuthError      = 3
	ExitAPIError       = 4
	ExitNotFound       = 5
	ExitCancelled      = 6
	ExitValidationFail = 7
	ExitTimeout        = 8
)
