package repository

type Repository struct {
	ID            int    `json:"repository_id"`
	Name          string `json:"repository_name"`
	Owner         string `json:"owner"`
	RepoName      string `json:"repo_name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	// Fields below are not provided by the BFF API but kept for compatibility
	HTMLURL     string `json:"html_url"`
	CloneURL    string `json:"clone_url"`
	SSHURL      string `json:"ssh_url"`
	Description string `json:"description"`
	Language    string `json:"language"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	PushedAt    string `json:"pushed_at"`
}

// GetRepositoriesResponse represents the response from the BFF repositories endpoint
// The API returns an array of repositories directly
type GetRepositoriesResponse struct {
	Repositories []Repository
	TotalCount   int
}

type RepositoryClient interface {
	GetGitHubRepositories(orgID string) (*GetRepositoriesResponse, error)
}
