package api

import (
	"time"
)

// An EnvironmentVariable has a Variable, a ContextID (its owner), and a
// CreatedAt date.
type EnvironmentVariable struct {
	Variable  string
	ContextID string
	CreatedAt time.Time
}

// A Context is the owner of EnvironmentVariables.
type Context struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
}

// ContextInterface is the interface to interact with contexts and environment
// variables.
type ContextInterface interface {
	Contexts(vcs, org string) (*[]Context, error)
	ContextByName(vcs, org, name string) (*Context, error)
	DeleteContext(contextID string) error
	CreateContext(vcs, org, name string) error

	EnvironmentVariables(contextID string) (*[]EnvironmentVariable, error)
	CreateEnvironmentVariable(contextID, variable, value string) error
	DeleteEnvironmentVariable(contextID, variable string) error
}
