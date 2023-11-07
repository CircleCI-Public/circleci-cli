package header

import "time"

// When the CLI is initialized, we set this to a string of the current CLI subcommand
// (e.g. `circleci orb list`) with no args or flags, so we include it as a header in API requests.
var cliCommandStr string = ""

func SetCommandStr(commandStr string) {
	cliCommandStr = commandStr
}

func GetCommandStr() string {
	return cliCommandStr
}

const defaultTimeout = 60 * time.Second

func GetDefaultTimeout() time.Duration {
	return defaultTimeout
}
