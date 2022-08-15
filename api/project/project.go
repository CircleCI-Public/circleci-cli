package project

// ProjectEnvironmentVariable is a Environment Variable of a Project
type ProjectEnvironmentVariable struct {
	Name  string
	Value string
}

// ProjectClient is the interface to interact with project and it's
// components.
type ProjectClient interface {
	ListAllEnvironmentVariables(vcs, org, project string) ([]*ProjectEnvironmentVariable, error)
}
