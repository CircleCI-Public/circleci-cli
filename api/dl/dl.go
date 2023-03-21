package dl

// ProjectClient is the interface to interact with dl
type DlClient interface {
	PurgeDLC(projectid string) error
}
