package data

type DataBag struct {
	Links struct {
		CLIDocs     string
		OrbDocs     string
		NewAPIToken string
	}
}

var Data = DataBag{
	Links: struct {
		CLIDocs     string
		OrbDocs     string
		NewAPIToken string
	}{
		CLIDocs:     "https://circleci.com/docs/2.0/local-cli/",
		OrbDocs:     "https://circleci.com/docs/2.0/orb-intro/",
		NewAPIToken: "https://circleci.com/account/api",
	},
}
