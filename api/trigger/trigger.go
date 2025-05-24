package trigger

type CreateTriggerInfo struct {
	Id   string
	Name string
}

type GetPipelineDefinitionOptions struct {
	ProjectID            string
	PipelineDefinitionID string
}

// TriggerClient is the interface to interact with trigger
type TriggerClient interface {
	CreateTrigger(options CreateTriggerOptions) (*CreateTriggerInfo, error)
	GetPipelineDefinition(options GetPipelineDefinitionOptions) (*PipelineDefinition, error)
}
