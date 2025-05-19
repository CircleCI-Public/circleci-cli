package pipeline

type CreatePipelineInfo struct {
	Id           string
	Name         string
	RepoFullName string
}

// PipelineClient is the interface to interact with pipeline and it's
// components.
type PipelineClient interface {
	CreatePipeline(projectID string, name string, description string, repoID string, filePath string) (*CreatePipelineInfo, error)
}
