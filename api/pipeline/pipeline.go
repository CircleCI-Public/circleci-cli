package pipeline

type CreatePipelineInfo struct {
	Id                         string
	Name                       string
	CheckoutSourceRepoFullName string
	ConfigSourceRepoFullName   string
}

type PipelineDefinition struct {
	ConfigSourceId   string
	CheckoutSourceId string
}

type GetPipelineDefinitionOptions struct {
	ProjectID            string
	PipelineDefinitionID string
}

// PipelineClient is the interface to interact with pipeline and it's
// components.
type PipelineClient interface {
	CreatePipeline(projectID string, name string, description string, repoID string, configRepoID string, filePath string) (*CreatePipelineInfo, error)
	GetPipelineDefinition(options GetPipelineDefinitionOptions) (*PipelineDefinition, error)
	ListPipelineDefinitions(projectID string) ([]*PipelineDefinitionInfo, error)
}
