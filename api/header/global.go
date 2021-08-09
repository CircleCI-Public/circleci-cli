package header

// When the CLI is initialized, we set this to a string of the current CLI subcommand
// (e.g. `orb list`) with no args or flags, so we include it as a header in API requests.
var cliCommandStr string = ""

func SetCommandStr(commandStr string) {
	cliCommandStr = commandStr
}

func GetCommandStr() string {
	return cliCommandStr
}
