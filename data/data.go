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
		CLIDocs:     "https://circleci.com/docs/guides/toolkit/local-cli/",
		OrbDocs:     "https://circleci.com/docs/orbs/use/orb-intro/",
		NewAPIToken: "https://circleci.com/account/api",
	},
}
