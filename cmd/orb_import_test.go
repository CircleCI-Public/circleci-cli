package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Import unit testing", func() {
	var (
		cli    *clitest.TempSettings
		client *graphql.Client
	)
	BeforeEach(func() {
		cli = clitest.WithTempSettings()
		client = cli.NewFakeClient("graphql-unstable", "")
	})

	AfterEach(func() {
		cli.Close()
	})

	Describe("When fetching all 'versionsToImport'", func() {
		It("should fail when fetching orb info", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  infoReq,
				Response: infoResp,
			})

			_, err := versionsToImport(opts)
			Expect(err).To(MatchError("orb info: no Orb 'namespace1/orb@0.0.1' was found; please check that the Orb reference is correct"))
		})

		It("should successfully fetch an orb version", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  infoReq,
				Response: infoResp,
			})

			v, err := versionsToImport(opts)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(v).To(Equal([]api.OrbVersion{
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
			}))
		})

		It("should fail when no namespace is found", func() {
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
				  }	`

			infoResp := `{}`

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  infoReq,
				Response: infoResp,
			})

			_, err := versionsToImport(opts)
			Expect(err).To(MatchError("list namespace orb versions: No namespace found"))
		})

		It("should successfully fetch all orb versions in a namespace", func() {
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
				  }	`

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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  listReq,
				Response: listResp,
			})

			v, err := versionsToImport(opts)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(v).To(Equal([]api.OrbVersion{
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
			}))
		})

		It("should successfully fetch all orb versions from multiple arguments", func() {
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
				  }	`

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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  listReq,
				Response: listResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  infoReq,
				Response: infoResp,
			})

			v, err := versionsToImport(opts)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(v).To(Equal([]api.OrbVersion{
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
			}))
		})
	})

	Describe("When testing 'generateImportPlan'", func() {
		var opts orbOptions

		BeforeEach(func() {
			opts = orbOptions{
				cl:                 client,
				cfg:                &settings.Config{},
				args:               []string{"namespace1/orb@0.0.1"},
				integrationTesting: true,
			}
		})
		It("should generate a plan with all resources", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  nsReq,
				Response: nsResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  orbExistsReq,
				Response: orbExistsResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  orbInfoReq,
				Response: orbInfoResp,
			})

			plan, err := generateImportPlan(opts, vs)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(plan).To(Equal(orbImportPlan{
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
			}))
		})

		It("should generate a plan with overlapping orbs and namespaces", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  fmt.Sprintf(nsReq, "namespace1"),
				Response: nsResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  fmt.Sprintf(orbExistsReq, "namespace1/orb", "namespace1"),
				Response: orbExistsResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  fmt.Sprintf(orbInfoReq, "namespace1/orb@0.0.1"),
				Response: orbInfoResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  fmt.Sprintf(orbInfoReq, "namespace1/orb@0.0.2"),
				Response: orbInfoResp,
			})

			plan, err := generateImportPlan(opts, vs)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(plan).To(Equal(orbImportPlan{
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
			}))
		})

		It("should generate an action plan when some resources exist", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  fmt.Sprintf(nsReq, "namespace1"),
				Response: nsResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  fmt.Sprintf(orbExistsReq, "namespace1/orb", "namespace1"),
				Response: orbExistsResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  fmt.Sprintf(orbInfoReq, "namespace1/orb@0.0.1"),
				Response: orbInfoResp,
			})

			plan, err := generateImportPlan(opts, vs)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(plan).To(Equal(orbImportPlan{
				AlreadyExistingVersions: []api.OrbVersion{
					{
						ID:        "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						Version:   "0.0.1",
						Orb:       api.Orb{Name: "namespace1/orb", Namespace: api.Namespace{Name: "namespace1"}},
						Source:    "description: somesource",
						CreatedAt: "2018-09-24T08:53:37.086Z",
					},
				},
			}))
		})
	})

	Describe("When testing 'displayPlan'", func() {
		It("prints out plan correctly to the provided writer", func() {
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
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(actual)).To(Equal(expOutput))
		})
	})

	Describe("When testing 'applyPlan'", func() {
		var opts orbOptions

		BeforeEach(func() {
			opts = orbOptions{
				cl:                 client,
				cfg:                &settings.Config{},
				args:               []string{"namespace1/orb@0.0.1"},
				integrationTesting: true,
			}
		})

		It("errors when creating an imported namespace", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  createNSReq,
				Response: createNSResp,
			})

			err := applyPlan(opts, plan)
			Expect(err).To(MatchError("unable to create 'namespace1' namespace: testerror"))
		})

		It("successfully creates a namespace", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  createNSReq,
				Response: createNSResp,
			})

			err := applyPlan(opts, plan)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("errors when creating an orb and namespace id fetch fails", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  getNSReq,
				Response: getNSResp,
			})

			err := applyPlan(opts, plan)
			Expect(err).To(MatchError("unable to create 'namespace1/orb' orb: the namespace 'namespace1' does not exist. Did you misspell the namespace, or maybe you meant to create the namespace first?"))
		})

		It("errors when creating an orb", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  getNSReq,
				Response: getNSResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  importOrbReq,
				Response: importOrbResp,
			})

			err := applyPlan(opts, plan)
			Expect(err).To(MatchError("unable to create 'namespace1/orb' orb: testerror"))
		})

		It("creates an orb successfully", func() {
			plan := orbImportPlan{
				NewOrbs: []api.Orb{
					{
						Name: "namespace1/orb",

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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  getNSReq,
				Response: getNSResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  createOrbReq,
				Response: createOrbResp,
			})

			err := applyPlan(opts, plan)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("fails to publish an orb version with source", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  orbIDReq,
				Response: orbIDResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  orbPublishReq,
				Response: orbPublishResp,
			})

			err := applyPlan(opts, plan)
			Expect(err).To(MatchError("unable to publish 'namespace1/orb@0.0.1': ERROR IN CONFIG FILE:\ntesterror\nThis can be caused by an orb using syntax that is not supported on your server version."))
		})

		It("fails to publish an orb version with source", func() {
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

			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  orbIDReq,
				Response: orbIDResp,
			})
			cli.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  orbPublishReq,
				Response: orbPublishResp,
			})

			err := applyPlan(opts, plan)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
