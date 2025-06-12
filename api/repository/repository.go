package repository

// Repository represents a GitHub repository from the BFF API
type Repository struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	PushedAt      string `json:"pushed_at"`
	DefaultBranch string `json:"default_branch"`
}

// GetRepositoriesResponse represents the response from the BFF repositories endpoint
// The API returns an array of repositories directly
type GetRepositoriesResponse struct {
	Repositories []Repository
	TotalCount   int
}

// RepositoryClient is the interface to interact with repository-related APIs
type RepositoryClient interface {
	GetGitHubRepositories(orgID string) (*GetRepositoriesResponse, error)
}
