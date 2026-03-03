package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

// respondGQLDataInternal returns an http.HandlerFunc that writes a JSON response
// wrapped in {"data": ...}, matching what clitest.AppendPostHandler did.
// This is defined here since package cmd cannot see the cmd_test helpers.
func respondGQLDataInternal(status int, jsonBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{ "data": %s}`, jsonBody)
	}
}

func newFakeClient(t testing.TB, server *testhelpers.TestServer) *graphql.Client {
	t.Helper()
	return graphql.NewClient(http.DefaultClient, server.URL, "graphql-unstable", "", false)
}

func TestVersionsToImport_FailFetchingOrbInfo(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	infoReq := `{
		"query": "query($orbVersionRef: String!) {\n\t\t\t    orbVersion(orbVersionRef: $orbVersionRef) {\n\t\t\t        id\n                                version\n                                orb {\n                                    id\n                                    createdAt\n\t\t\t\t\t\t\t\t\tname\n\t\t\t\t\t\t\t\t\tnamespace {\n\t\t\t\t\t\t\t\t\t  name\n\t\t\t\t\t\t\t\t\t}\n                                    categories {\n                                      id\n                                      name\n                                    }\n\t                            statistics {\n\t\t                        last30DaysBuildCount,\n\t\t                        last30DaysProjectCount,\n\t\t                        last30DaysOrganizationCount\n\t                            }\n                                    versions(count: 200) {\n                                        createdAt\n                                        version\n                                    }\n                                }\n                                source\n                                createdAt\n\t\t\t    }\n\t\t      }",
		"variables": {
		  "orbVersionRef": "namespace1/orb@0.0.1"
		}
	}`
	infoResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", infoReq),
		respondGQLDataInternal(http.StatusOK, infoResp),
	))

	_, err := versionsToImport(opts)
	assert.ErrorContains(t, err, "no Orb 'namespace1/orb@0.0.1' was found")
}

func TestVersionsToImport_SuccessfullyFetchOrbVersion(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	infoReq := `{
		"query": "query($orbVersionRef: String!) {\n\t\t\t    orbVersion(orbVersionRef: $orbVersionRef) {\n\t\t\t        id\n                                version\n                                orb {\n                                    id\n                                    createdAt\n\t\t\t\t\t\t\t\t\tname\n\t\t\t\t\t\t\t\t\tnamespace {\n\t\t\t\t\t\t\t\t\t  name\n\t\t\t\t\t\t\t\t\t}\n                                    categories {\n                                      id\n                                      name\n                                    }\n\t                            statistics {\n\t\t                        last30DaysBuildCount,\n\t\t                        last30DaysProjectCount,\n\t\t                        last30DaysOrganizationCount\n\t                            }\n                                    versions(count: 200) {\n                                        createdAt\n                                        version\n                                    }\n                                }\n                                source\n                                createdAt\n\t\t\t    }\n\t\t      }",
		"variables": {
		  "orbVersionRef": "namespace1/orb@0.0.1"
		}
	}`

	infoResp := `{
		"orbVersion": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			"version": "0.0.1",
			"orb": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				"createdAt": "2018-09-24T08:53:37.086Z",
				"name": "namespace1/orb",
				"namespace": {
					"name": "namespace1"
				},
				"versions": [
					{
						"version": "0.0.1",
						"createdAt": "2018-10-11T22:12:19.477Z"
					}
				]
			},
			"source": "description: somesource",
			"createdAt": "2018-09-24T08:53:37.086Z"
		}
	}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", infoReq),
		respondGQLDataInternal(http.StatusOK, infoResp),
	))

	v, err := versionsToImport(opts)
	assert.NilError(t, err)
	assert.DeepEqual(t, v, []api.OrbVersion{
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.1",
			Source:  "description: somesource",
			Orb: api.Orb{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
				Versions: []api.OrbVersion{
					{Version: "0.0.1", CreatedAt: "2018-10-11T22:12:19.477Z"},
				},
				CreatedAt:      "2018-09-24T08:53:37.086Z",
				HighestVersion: "0.0.1",
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
	})
}

func TestVersionsToImport_NoNamespaceFound(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1"},
		integrationTesting: true,
	}

	infoReq := `{
		"query": "\nquery namespaceOrbs ($namespace: String, $after: String!) {\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tname\n\t\t\tid\n\t\t\torbs(first: 20, after: $after) {\n\t\t\t\tedges {\n\t\t\t\t\tcursor\n\t\t\t\t\tnode {\n\t\t\t\t\t\tversions(count: 1) {\n\t\t\t\t\t\t\tsource\n\t\t\t\t\t\t\tid\n\t\t\t\t\t\t\tversion\n\t\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t\t}\n\t\t\t\t\t\tname\n\t\t\t\t\t\tid\n\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tpageInfo {\n\t\t\t\t\thasNextPage\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n",
		"variables": {
		  "after": "",
		  "namespace": "namespace1"
		}
	}`

	infoResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", infoReq),
		respondGQLDataInternal(http.StatusOK, infoResp),
	))

	_, err := versionsToImport(opts)
	assert.ErrorContains(t, err, "no namespace found")
}

func TestVersionsToImport_FetchAllOrbVersionsInNamespace(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1"},
		integrationTesting: true,
	}

	listReq := `{
		"query": "\nquery namespaceOrbs ($namespace: String, $after: String!) {\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tname\n\t\t\tid\n\t\t\torbs(first: 20, after: $after) {\n\t\t\t\tedges {\n\t\t\t\t\tcursor\n\t\t\t\t\tnode {\n\t\t\t\t\t\tversions(count: 1) {\n\t\t\t\t\t\t\tsource\n\t\t\t\t\t\t\tid\n\t\t\t\t\t\t\tversion\n\t\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t\t}\n\t\t\t\t\t\tname\n\t\t\t\t\t\tid\n\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tpageInfo {\n\t\t\t\t\thasNextPage\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n",
		"variables": {
		  "after": "",
		  "namespace": "namespace1"
		}
	}`

	listResp := `{
		"registryNamespace": {
			"name": "namespace1",
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			"orbs": {
				"edges": [
					{
						"node": {
							"name": "namespace1/orb",
							"versions": [
								{
									"source": "description: somesource",
									"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
									"version": "0.0.1",
									"createdAt": "2018-09-24T08:53:37.086Z"
								},
								{
									"source": "description: someothersource",
									"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
									"version": "0.0.2",
									"createdAt": "2018-09-24T08:53:37.086Z"
								}
							]
						}
					}
				]
			}
		}
	}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", listReq),
		respondGQLDataInternal(http.StatusOK, listResp),
	))

	v, err := versionsToImport(opts)
	assert.NilError(t, err)
	assert.DeepEqual(t, v, []api.OrbVersion{
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.1",
			Source:  "description: somesource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.2",
			Source:  "description: someothersource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
	})
}

func TestVersionsToImport_FetchFromMultipleArguments(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1", "namespace2/orb2@3.3.3"},
		integrationTesting: true,
	}

	listReq := `{
		"query": "\nquery namespaceOrbs ($namespace: String, $after: String!) {\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tname\n\t\t\tid\n\t\t\torbs(first: 20, after: $after) {\n\t\t\t\tedges {\n\t\t\t\t\tcursor\n\t\t\t\t\tnode {\n\t\t\t\t\t\tversions(count: 1) {\n\t\t\t\t\t\t\tsource\n\t\t\t\t\t\t\tid\n\t\t\t\t\t\t\tversion\n\t\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t\t}\n\t\t\t\t\t\tname\n\t\t\t\t\t\tid\n\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tpageInfo {\n\t\t\t\t\thasNextPage\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n",
		"variables": {
		  "after": "",
		  "namespace": "namespace1"
		}
	}`

	listResp := `{
		"registryNamespace": {
			"name": "namespace1",
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			"orbs": {
				"edges": [
					{
						"node": {
							"name": "namespace1/orb",
							"versions": [
								{
									"source": "description: somesource",
									"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
									"version": "0.0.1",
									"createdAt": "2018-09-24T08:53:37.086Z"
								},
								{
									"source": "description: someothersource",
									"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
									"version": "0.0.2",
									"createdAt": "2018-09-24T08:53:37.086Z"
								}
							]
						}
					}
				]
			}
		}
	}`

	infoReq := `{
		"query": "query($orbVersionRef: String!) {\n\t\t\t    orbVersion(orbVersionRef: $orbVersionRef) {\n\t\t\t        id\n                                version\n                                orb {\n                                    id\n                                    createdAt\n\t\t\t\t\t\t\t\t\tname\n\t\t\t\t\t\t\t\t\tnamespace {\n\t\t\t\t\t\t\t\t\t  name\n\t\t\t\t\t\t\t\t\t}\n                                    categories {\n                                      id\n                                      name\n                                    }\n\t                            statistics {\n\t\t                        last30DaysBuildCount,\n\t\t                        last30DaysProjectCount,\n\t\t                        last30DaysOrganizationCount\n\t                            }\n                                    versions(count: 200) {\n                                        createdAt\n                                        version\n                                    }\n                                }\n                                source\n                                createdAt\n\t\t\t    }\n\t\t      }",
		"variables": {
		  "orbVersionRef": "namespace2/orb2@3.3.3"
		}
	}`

	infoResp := `{
		"orbVersion": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			"version": "3.3.3",
			"orb": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				"createdAt": "2018-09-24T08:53:37.086Z",
				"name": "namespace2/orb2",
				"namespace": {
					"name": "namespace2"
				},
				"versions": [
					{
						"version": "3.3.3",
						"createdAt": "2018-10-11T22:12:19.477Z"
					}
				]
			},
			"source": "description: somesource",
			"createdAt": "2018-09-24T08:53:37.086Z"
		}
	}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", listReq),
		respondGQLDataInternal(http.StatusOK, listResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", infoReq),
		respondGQLDataInternal(http.StatusOK, infoResp),
	))

	v, err := versionsToImport(opts)
	assert.NilError(t, err)
	assert.DeepEqual(t, v, []api.OrbVersion{
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.1",
			Source:  "description: somesource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.2",
			Source:  "description: someothersource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "3.3.3",
			Source:  "description: somesource",
			Orb: api.Orb{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Name:      "namespace2/orb2",
				Namespace: api.Namespace{Name: "namespace2"},
				Versions: []api.OrbVersion{
					{Version: "3.3.3", CreatedAt: "2018-10-11T22:12:19.477Z"},
				},
				CreatedAt:      "2018-09-24T08:53:37.086Z",
				HighestVersion: "3.3.3",
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
	})
}

func TestGenerateImportPlan_AllResources(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	vs := []api.OrbVersion{
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.1",
			Source:  "description: somesource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
	}

	nsReq := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "namespace1"
		}
	}`
	nsResp := `{}`

	orbExistsReq := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
		  "name": "namespace1/orb",
		  "namespace": "namespace1"
		}
	}`
	orbExistsResp := `{}`

	orbInfoReq := `{
		"query": "query($orbVersionRef: String!) {\n\t\t\t    orbVersion(orbVersionRef: $orbVersionRef) {\n\t\t\t        id\n                                version\n                                orb {\n                                    id\n                                    createdAt\n\t\t\t\t\t\t\t\t\tname\n\t\t\t\t\t\t\t\t\tnamespace {\n\t\t\t\t\t\t\t\t\t  name\n\t\t\t\t\t\t\t\t\t}\n                                    categories {\n                                      id\n                                      name\n                                    }\n\t                            statistics {\n\t\t                        last30DaysBuildCount,\n\t\t                        last30DaysProjectCount,\n\t\t                        last30DaysOrganizationCount\n\t                            }\n                                    versions(count: 200) {\n                                        createdAt\n                                        version\n                                    }\n                                }\n                                source\n                                createdAt\n\t\t\t    }\n\t\t      }",
		"variables": {
		  "orbVersionRef": "namespace1/orb@0.0.1"
		}
	}`
	orbInfoResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", nsReq),
		respondGQLDataInternal(http.StatusOK, nsResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", orbExistsReq),
		respondGQLDataInternal(http.StatusOK, orbExistsResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", orbInfoReq),
		respondGQLDataInternal(http.StatusOK, orbInfoResp),
	))

	plan, err := generateImportPlan(opts, vs)
	assert.NilError(t, err)
	assert.DeepEqual(t, plan, orbImportPlan{
		NewNamespaces: []string{"namespace1"},
		NewOrbs: []api.Orb{
			{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
		},
		NewVersions: []api.OrbVersion{
			{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Version:   "0.0.1",
				Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
				Source:    "description: somesource",
				CreatedAt: "2018-09-24T08:53:37.086Z",
			},
		},
	})
}

func TestGenerateImportPlan_OverlappingOrbsAndNamespaces(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	vs := []api.OrbVersion{
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.1",
			Source:  "description: somesource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.2",
			Source:  "description: somesource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
	}

	nsReq := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "%s"
		}
	}`
	nsResp := `{}`

	orbExistsReq := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
		  "name": "%s",
		  "namespace": "%s"
		}
	}`
	orbExistsResp := `{}`

	orbInfoReq := `{
		"query": "query($orbVersionRef: String!) {\n\t\t\t    orbVersion(orbVersionRef: $orbVersionRef) {\n\t\t\t        id\n                                version\n                                orb {\n                                    id\n                                    createdAt\n\t\t\t\t\t\t\t\t\tname\n\t\t\t\t\t\t\t\t\tnamespace {\n\t\t\t\t\t\t\t\t\t  name\n\t\t\t\t\t\t\t\t\t}\n                                    categories {\n                                      id\n                                      name\n                                    }\n\t                            statistics {\n\t\t                        last30DaysBuildCount,\n\t\t                        last30DaysProjectCount,\n\t\t                        last30DaysOrganizationCount\n\t                            }\n                                    versions(count: 200) {\n                                        createdAt\n                                        version\n                                    }\n                                }\n                                source\n                                createdAt\n\t\t\t    }\n\t\t      }",
		"variables": {
		  "orbVersionRef": "%s"
		}
	}`
	orbInfoResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", fmt.Sprintf(nsReq, "namespace1")),
		respondGQLDataInternal(http.StatusOK, nsResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", fmt.Sprintf(orbExistsReq, "namespace1/orb", "namespace1")),
		respondGQLDataInternal(http.StatusOK, orbExistsResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", fmt.Sprintf(orbInfoReq, "namespace1/orb@0.0.1")),
		respondGQLDataInternal(http.StatusOK, orbInfoResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", fmt.Sprintf(orbInfoReq, "namespace1/orb@0.0.2")),
		respondGQLDataInternal(http.StatusOK, orbInfoResp),
	))

	plan, err := generateImportPlan(opts, vs)
	assert.NilError(t, err)
	assert.DeepEqual(t, plan, orbImportPlan{
		NewNamespaces: []string{"namespace1"},
		NewOrbs: []api.Orb{
			{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
		},
		NewVersions: []api.OrbVersion{
			{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Version:   "0.0.1",
				Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
				Source:    "description: somesource",
				CreatedAt: "2018-09-24T08:53:37.086Z",
			},
			{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Version:   "0.0.2",
				Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
				Source:    "description: somesource",
				CreatedAt: "2018-09-24T08:53:37.086Z",
			},
		},
	})
}

func TestGenerateImportPlan_SomeResourcesExist(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	vs := []api.OrbVersion{
		{
			ID:      "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			Version: "0.0.1",
			Source:  "description: somesource",
			Orb: api.Orb{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
			CreatedAt: "2018-09-24T08:53:37.086Z",
		},
	}

	nsReq := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "%s"
		}
	}`
	nsResp := `{
		"registryNamespace": {
			"id": "someid"
		}
	}`

	orbExistsReq := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
		  "name": "%s",
		  "namespace": "%s"
		}
	}`
	orbExistsResp := `{
		"orb": {
			"id": "someid",
			"isPrivate": false
		}
	}`

	orbInfoReq := `{
		"query": "query($orbVersionRef: String!) {\n\t\t\t    orbVersion(orbVersionRef: $orbVersionRef) {\n\t\t\t        id\n                                version\n                                orb {\n                                    id\n                                    createdAt\n\t\t\t\t\t\t\t\t\tname\n\t\t\t\t\t\t\t\t\tnamespace {\n\t\t\t\t\t\t\t\t\t  name\n\t\t\t\t\t\t\t\t\t}\n                                    categories {\n                                      id\n                                      name\n                                    }\n\t                            statistics {\n\t\t                        last30DaysBuildCount,\n\t\t                        last30DaysProjectCount,\n\t\t                        last30DaysOrganizationCount\n\t                            }\n                                    versions(count: 200) {\n                                        createdAt\n                                        version\n                                    }\n                                }\n                                source\n                                createdAt\n\t\t\t    }\n\t\t      }",
		"variables": {
		  "orbVersionRef": "%s"
		}
	}`
	orbInfoResp := `{
		"orbVersion": {
			"id": "someid"
		}
	}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", fmt.Sprintf(nsReq, "namespace1")),
		respondGQLDataInternal(http.StatusOK, nsResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", fmt.Sprintf(orbExistsReq, "namespace1/orb", "namespace1")),
		respondGQLDataInternal(http.StatusOK, orbExistsResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", fmt.Sprintf(orbInfoReq, "namespace1/orb@0.0.1")),
		respondGQLDataInternal(http.StatusOK, orbInfoResp),
	))

	plan, err := generateImportPlan(opts, vs)
	assert.NilError(t, err)
	assert.DeepEqual(t, plan, orbImportPlan{
		AlreadyExistingVersions: []api.OrbVersion{
			{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Version:   "0.0.1",
				Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
				Source:    "description: somesource",
				CreatedAt: "2018-09-24T08:53:37.086Z",
			},
		},
	})
}

func TestDisplayPlan(t *testing.T) {
	plan := orbImportPlan{
		NewNamespaces: []string{"namespace1"},
		NewOrbs: []api.Orb{
			{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
		},
		NewVersions: []api.OrbVersion{
			{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Version:   "0.0.1",
				Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
				Source:    "description: somesource",
				CreatedAt: "2018-09-24T08:53:37.086Z",
			},
			{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Version:   "0.0.2",
				Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
				Source:    "description: somesource",
				CreatedAt: "2018-09-24T08:53:37.086Z",
			},
		},
		AlreadyExistingVersions: []api.OrbVersion{
			{
				ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				Version:   "0.0.3",
				Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
				Source:    "description: somesource",
				CreatedAt: "2018-09-24T08:53:37.086Z",
			},
		},
	}

	var b bytes.Buffer
	displayPlan(&b, plan)

	expOutput := `The following actions will be performed:
  Create namespace 'namespace1'
  Create orb 'namespace1/orb'
  Import version 'namespace1/orb@0.0.1'
  Import version 'namespace1/orb@0.0.2'

The following orb versions already exist:
  ('namespace1/orb@0.0.3')

`
	actual, err := io.ReadAll(&b)
	assert.NilError(t, err)
	assert.Equal(t, string(actual), expOutput)
}

func TestApplyPlan_ErrorCreatingNamespace(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	plan := orbImportPlan{
		NewNamespaces: []string{"namespace1"},
	}

	createNSReq := `{
		"query": "\n\t\t\tmutation($name: String!) {\n\t\t\t\timportNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
		"variables": {
		  "name": "namespace1"
		}
	}`
	createNSResp := `{
		"importNamespace": {
			"errors": [{"message": "testerror"}]
		}
	}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", createNSReq),
		respondGQLDataInternal(http.StatusOK, createNSResp),
	))

	err := applyPlan(opts, plan)
	assert.ErrorContains(t, err, "unable to create 'namespace1' namespace: testerror")
}

func TestApplyPlan_SuccessCreatingNamespace(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	plan := orbImportPlan{
		NewNamespaces: []string{"namespace1"},
	}

	createNSReq := `{
		"query": "\n\t\t\tmutation($name: String!) {\n\t\t\t\timportNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
		"variables": {
		  "name": "namespace1"
		}
	}`
	createNSResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", createNSReq),
		respondGQLDataInternal(http.StatusOK, createNSResp),
	))

	err := applyPlan(opts, plan)
	assert.NilError(t, err)
}

func TestApplyPlan_ErrorCreatingOrb_NamespaceFetchFails(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	plan := orbImportPlan{
		NewOrbs: []api.Orb{
			{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
		},
	}

	getNSReq := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "namespace1"
		}
	}`
	getNSResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", getNSReq),
		respondGQLDataInternal(http.StatusOK, getNSResp),
	))

	err := applyPlan(opts, plan)
	assert.ErrorContains(t, err, "unable to create 'namespace1/orb' orb: the namespace 'namespace1' does not exist")
}

func TestApplyPlan_ErrorCreatingOrb(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	plan := orbImportPlan{
		NewOrbs: []api.Orb{
			{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
		},
	}

	getNSReq := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "namespace1"
		}
	}`
	getNSResp := `{
		"registryNamespace": {
			"id": "someid1"
		}
	}`

	importOrbReq := `{
		"query": "mutation($name: String!, $registryNamespaceId: UUID!){\n\t\t\t\timportOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
		"variables": {
		  "name": "orb",
		  "registryNamespaceId": "someid1"
		}
	}`
	importOrbResp := `{
		"importOrb": {
			"errors": [{"message": "testerror"}]
		}
	}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", getNSReq),
		respondGQLDataInternal(http.StatusOK, getNSResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", importOrbReq),
		respondGQLDataInternal(http.StatusOK, importOrbResp),
	))

	err := applyPlan(opts, plan)
	assert.ErrorContains(t, err, "unable to create 'namespace1/orb' orb: testerror")
}

func TestApplyPlan_SuccessCreatingOrb(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	plan := orbImportPlan{
		NewOrbs: []api.Orb{
			{
				Name:      "namespace1/orb",
				Namespace: api.Namespace{Name: "namespace1"},
			},
		},
	}

	getNSReq := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "namespace1"
		}
	}`
	getNSResp := `{
		"registryNamespace": {
			"id": "someid1"
		}
	}`

	createOrbReq := `{
		"query": "mutation($name: String!, $registryNamespaceId: UUID!){\n\t\t\t\timportOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
		"variables": {
		  "name": "orb",
		  "registryNamespaceId": "someid1"
		}
	}`
	createOrbResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", getNSReq),
		respondGQLDataInternal(http.StatusOK, getNSResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", createOrbReq),
		respondGQLDataInternal(http.StatusOK, createOrbResp),
	))

	err := applyPlan(opts, plan)
	assert.NilError(t, err)
}

func TestApplyPlan_FailPublishOrbVersion(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	plan := orbImportPlan{
		NewVersions: []api.OrbVersion{
			{
				Version: "0.0.1",
				Orb:     api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
			},
		},
	}

	orbIDReq := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
		"variables": {
		  "name": "namespace1/orb",
		  "namespace": "namespace1"
		}
	}`
	orbIDResp := `{
		"orb": {"id": "orbid1"}
	}`

	orbPublishReq := `{
		"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\timportOrbVersion(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
		  "config": "",
		  "orbId": "orbid1",
		  "version": "0.0.1"
		}
	}`
	orbPublishResp := `{
		"importOrbVersion": {
			"errors": [{"message": "ERROR IN CONFIG FILE:\ntesterror"}]
		}
	}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", orbIDReq),
		respondGQLDataInternal(http.StatusOK, orbIDResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", orbPublishReq),
		respondGQLDataInternal(http.StatusOK, orbPublishResp),
	))

	err := applyPlan(opts, plan)
	assert.ErrorContains(t, err, "unable to publish 'namespace1/orb@0.0.1': ERROR IN CONFIG FILE:\ntesterror")
}

func TestApplyPlan_SuccessPublishOrbVersion(t *testing.T) {
	server := testhelpers.NewTestServer(t)
	client := newFakeClient(t, server)

	opts := orbOptions{
		cl:                 client,
		cfg:                &settings.Config{},
		args:               []string{"namespace1/orb@0.0.1"},
		integrationTesting: true,
	}

	plan := orbImportPlan{
		NewVersions: []api.OrbVersion{
			{
				Version: "0.0.1",
				Orb:     api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
			},
		},
	}

	orbIDReq := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
		"variables": {
		  "name": "namespace1/orb",
		  "namespace": "namespace1"
		}
	}`
	orbIDResp := `{
		"orb": {"id": "orbid1"}
	}`

	orbPublishReq := `{
		"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\timportOrbVersion(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
		  "config": "",
		  "orbId": "orbid1",
		  "version": "0.0.1"
		}
	}`
	orbPublishResp := `{}`

	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", orbIDReq),
		respondGQLDataInternal(http.StatusOK, orbIDResp),
	))
	server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, "", orbPublishReq),
		respondGQLDataInternal(http.StatusOK, orbPublishResp),
	))

	err := applyPlan(opts, plan)
	assert.NilError(t, err)
}
