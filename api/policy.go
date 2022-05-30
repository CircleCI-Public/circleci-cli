package api

type PolicyInterface interface {
	ListPolicies(ownerID, activeFilter string) (string, error)
}
