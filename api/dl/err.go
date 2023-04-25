package dl

type CloudOnlyErr struct{}

func (e *CloudOnlyErr) Error() string {
	return "Misconfiguration.\n" +
		"You have configured a custom API endpoint host for the circleci CLI.\n" +
		"However, this functionality is only supported on circleci.com API endpoints."
}

func IsCloudOnlyErr(err error) bool {
	_, ok := err.(*CloudOnlyErr)
	return ok
}

type GoneErr struct{}

func (e *GoneErr) Error() string {
	return "No longer supported.\n" +
		"This functionality is no longer supported by this version of the circleci CLI.\n" +
		"Please upgrade to the latest version of the circleci CLI."
}

func IsGoneErr(err error) bool {
	_, ok := err.(*GoneErr)
	return ok
}
