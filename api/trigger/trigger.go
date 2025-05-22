package trigger

type CreateTriggerInfo struct {
	Id   string
	Name string
}

// TriggerClient is the interface to interact with trigger
type TriggerClient interface {
	CreateTrigger(options CreateTriggerOptions) (*CreateTriggerInfo, error)
}
