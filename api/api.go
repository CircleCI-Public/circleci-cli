
			CreateNamespaceResponse
		}
	}

	query := `
			mutation($name: String!, $organizationId: UUID!) {
				createNamespace(
					name: $name,
					organizationId: $organizationId
				) {
					namespace {
						id
					}
					errors {
						message
						type
					}
				}
			}`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("name", name)
	request.Var("organizationId", ownerID)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil {
		err = errors.Wrapf(err, "Unable to create namespace %s for ownerId %s", name, ownerID)
	}

	return &response.CreateNamespace.CreateNamespaceResponse, err
}

func getOrganization(ctx context.Context, logger *logger.Logger, organizationName string, organizationVcs string) (string, error) {
	var response struct {
		Organization struct {
			ID string
		}
	}

	query := `
			query($organizationName: String!, $organizationVcs: VCSType!) {
				organization(
					name: $organizationName
					vcsType: $organizationVcs
				) {
					id
				}
			}`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("organizationName", organizationName)
	request.Var("organizationVcs", organizationVcs)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return "", err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil || response.Organization.ID == "" {
		err = errors.Wrap(err, fmt.Sprintf("Unable to find organization %s of vcs-type %s", organizationName, organizationVcs))
	}

	return response.Organization.ID, err
}

func namespaceNotFound(name string) error {
	return fmt.Errorf("the namespace '%s' does not exist. Did you misspell the namespace, or maybe you meant to create the namespace first?", name)
}

// CreateNamespace creates (reserves) a namespace for an organization
func CreateNamespace(ctx context.Context, logger *logger.Logger, name string, organizationName string, organizationVcs string) (*CreateNamespaceResponse, error) {
	organizationID, err := getOrganization(ctx, logger, organizationName, organizationVcs)
	if err != nil {
		return nil, err
	}

	return createNamespaceWithOwnerID(ctx, logger, name, organizationID)
}

func getNamespace(ctx context.Context, logger *logger.Logger, name string) (string, error) {
	var response struct {
		RegistryNamespace struct {
			ID string
		}
	}

	query := `
				query($name: String!) {
					registryNamespace(
						name: $name
					){
						id
					}
			 }`
	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("name", name)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return "", err
	}
	graphQLclient := client.NewClient(address, logger)

	if err = graphQLclient.Run(ctx, request, &response); err != nil {
		return "", errors.Wrapf(err, "failed to load namespace '%s'", err)
	}

	if response.RegistryNamespace.ID == "" {
		return "", namespaceNotFound(name)
	}

	return response.RegistryNamespace.ID, nil
}

func createOrbWithNsID(ctx context.Context, logger *logger.Logger, name string, namespaceID string) (*CreateOrbResponse, error) {
	var response struct {
		CreateOrb struct {
			CreateOrbResponse
		}
	}

	query := `mutation($name: String!, $registryNamespaceId: UUID!){
				createOrb(
					name: $name,
					registryNamespaceId: $registryNamespaceId
				){
				    orb {
				      id
				    }
				    errors {
				      message
				      type
				    }
				}
}`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("name", name)
	request.Var("registryNamespaceId", namespaceID)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	return &response.CreateOrb.CreateOrbResponse, err
}

// CreateOrb creates (reserves) an orb within a namespace
func CreateOrb(ctx context.Context, logger *logger.Logger, namespace string, name string) (*CreateOrbResponse, error) {
	namespaceID, err := getNamespace(ctx, logger, namespace)
	if err != nil {
		return nil, err
	}

	return createOrbWithNsID(ctx, logger, name, namespaceID)
}

// TODO(zzak): this function is not really related to the API. Move it to another package?
func incrementVersion(version string, segment string) (string, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}

	var v2 semver.Version
	switch segment {
	case "major":
		v2 = v.IncMajor()
	case "minor":
		v2 = v.IncMinor()
	case "patch":
		v2 = v.IncPatch()
	}

	return v2.String(), nil
}

// OrbIncrementVersion accepts an orb and segment to increment the orb.
func OrbIncrementVersion(ctx context.Context, logger *logger.Logger, configPath string, namespace string, orb string, segment string) (*Orb, error) {
	id, err := OrbID(ctx, logger, namespace, orb)
	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(ctx, logger, namespace, orb)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	response, err := OrbPublishByID(ctx, logger, configPath, id, v2)
	if err != nil {
		return nil, err
	}

	logger.Debug("Bumped %s/%s#%s from %s by %s to %s\n.", namespace, orb, id, v, segment, v2)

	return response, nil
}

// OrbLatestVersion finds the latest published version of an orb and returns it.
// If it doesn't find a version, it will return 0.0.0 for the orb's version
func OrbLatestVersion(ctx context.Context, logger *logger.Logger, namespace string, orb string) (string, error) {
	name := namespace + "/" + orb

	var response struct {
		Orb struct {
			Versions []struct {
				Version string
			}
		}
	}

	// This query returns versions sorted by semantic version
	query := `query($name: String!) {
			    orb(name: $name) {
			      versions(count: 1) {
				    version
			      }
			    }
		      }`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("name", name)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return "", err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil {
		return "", err
	}

	if len(response.Orb.Versions) != 1 {
		return "0.0.0", nil
	}

	return response.Orb.Versions[0].Version, nil
}

// OrbPromote takes an orb and a development version and increments a semantic release with the given segment.
func OrbPromote(ctx context.Context, logger *logger.Logger, namespace string, orb string, label string, segment string) (*Orb, error) {
	id, err := OrbID(ctx, logger, namespace, orb)
	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(ctx, logger, namespace, orb)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	var response struct {
		PromoteOrb struct {
			OrbPromoteResponse
		}
	}

	query := `
		mutation($orbId: UUID!, $devVersion: String!, $semanticVersion: String!) {
			promoteOrb(
				orbId: $orbId,
				devVersion: $devVersion,
				semanticVersion: $semanticVersion
			) {
				orb {
					version
					source
				}
				errors { message }
			}
		}
	`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("orbId", id)
	request.Var("devVersion", label)
	request.Var("semanticVersion", v2)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to promote orb")
	}

	if len(response.PromoteOrb.OrbPromoteResponse.Errors) > 0 {
		return nil, response.PromoteOrb.OrbPromoteResponse.ToError()
	}

	return &response.PromoteOrb.OrbPromoteResponse.Orb, err
}

// orbVersionRef is designed to ensure an orb reference fits the orbVersion query where orbVersionRef argument requires a version
func orbVersionRef(orb string) string {
	split := strings.Split(orb, "@")
	// We're expecting the API to tell us the reference is acceptable
	// Without performing a lot of client-side validation
	if len(split) > 1 {
		return orb
	}

	// If no version was supplied, append @volatile to the reference
	return fmt.Sprintf("%s@%s", split[0], "volatile")
}

// OrbSource gets the source or an orb
func OrbSource(ctx context.Context, logger *logger.Logger, orbRef string) (string, error) {
	ref := orbVersionRef(orbRef)

	var response struct {
		OrbVersion struct {
			ID      string
			Version string
			Orb     struct {
				ID string
			}
			Source string
		}
	}

	query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("orbVersionRef", ref)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return "", err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil {
		return "", err
	}

	if response.OrbVersion.ID == "" {
		return "", fmt.Errorf("the %s orb has never published a revision", orbRef)
	}

	return response.OrbVersion.Source, nil
}

// ListOrbs queries the API to find all orbs.
// Returns a collection of Orb objects containing their relevant data. Logs
// request and parse errors to the supplied logger.
func ListOrbs(ctx context.Context, logger *logger.Logger, uncertified bool) (*OrbCollection, error) {
	// Define a structure that matches the result of the GQL
	// query, so that we can use mapstructure to convert from
	// nested maps to a strongly typed struct.
	type orbList struct {
		Orbs struct {
			TotalCount int
			Edges      []struct {
				Cursor string
				Node   struct {
					Name     string
					Versions []struct {
						Version string
						Source  string
					}
				}
			}
			PageInfo struct {
				HasNextPage bool
			}
		}
	}

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
	`

	var orbs OrbCollection

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	var result orbList
	currentCursor := ""

	for {
		request := client.NewAuthorizedRequest(viper.GetString("token"), query)
		request.Var("after", currentCursor)
		request.Var("certifiedOnly", !uncertified)

		err := graphQLclient.Run(ctx, request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

	Orbs:
		for i := range result.Orbs.Edges {
			edge := result.Orbs.Edges[i]
			currentCursor = edge.Cursor
			if len(edge.Node.Versions) > 0 {
				v := edge.Node.Versions[0]

				var o Orb

				o.Name = edge.Node.Name
				o.HighestVersion = v.Version

				for _, v := range edge.Node.Versions {
					o.Versions = append(o.Versions, OrbVersion(v))
				}
				err := yaml.Unmarshal([]byte(edge.Node.Versions[0].Source), &o)

				if err != nil {
					logger.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
					continue Orbs
				}
				orbs.Orbs = append(orbs.Orbs, o)
			}
		}

		if !result.Orbs.PageInfo.HasNextPage {
			break
		}
	}
	return &orbs, nil
}

// ListNamespaceOrbs queries the API to find all orbs belonging to the given
// namespace.
// Returns a collection of Orb objects containing their relevant data. Logs
// request and parse errors to the supplied logger.
func ListNamespaceOrbs(ctx context.Context, logger *logger.Logger, namespace string) (*OrbCollection, error) {
	// Define a structure that matches the result of the GQL
	// query, so that we can use mapstructure to convert from
	// nested maps to a strongly typed struct.
	type namespaceOrbResponse struct {
		RegistryNamespace struct {
			Name string
			Orbs struct {
				Edges []struct {
					Cursor string
					Node   struct {
						Name     string
						Versions []struct {
							Version string
							Source  string
						}
					}
				}
				TotalCount int
				PageInfo   struct {
					HasNextPage bool
				}
			}
		}
	}

	query := `
query namespaceOrbs ($namespace: String, $after: String!) {
	registryNamespace(name: $namespace) {
		name
		orbs(first: 20, after: $after) {
			edges {
				cursor
				node {
					versions {
						source
						version
					}
					name
				}
			}
			totalCount
			pageInfo {
				hasNextPage
			}
		}
	}
}
`
	var orbs OrbCollection

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	var result namespaceOrbResponse
	currentCursor := ""

	for {
		request := client.NewAuthorizedRequest(viper.GetString("token"), query)
		request.Var("after", currentCursor)
		request.Var("namespace", namespace)
		orbs.Namespace = namespace

		err := graphQLclient.Run(ctx, request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

	NamespaceOrbs:
		for i := range result.RegistryNamespace.Orbs.Edges {
			edge := result.RegistryNamespace.Orbs.Edges[i]
			currentCursor = edge.Cursor
			if len(edge.Node.Versions) > 0 {
				v := edge.Node.Versions[0]

				// Parse the orb source to print its commands, executors and jobs
				var o Orb
				o.Name = edge.Node.Name
				o.HighestVersion = v.Version
				for _, v := range edge.Node.Versions {
					o.Versions = append(o.Versions, OrbVersion(v))
				}
				err := yaml.Unmarshal([]byte(edge.Node.Versions[0].Source), &o)
				if err != nil {
					logger.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
					continue NamespaceOrbs
				}
				orbs.Orbs = append(orbs.Orbs, o)
			}
		}

		if !result.RegistryNamespace.Orbs.PageInfo.HasNextPage {
			break
		}
	}

	return &orbs, nil
}

// IntrospectionQuery makes a query on the API asking for bits of the schema
// This query isn't intended to get the entire schema, there are better tools for that.
func IntrospectionQuery(ctx context.Context, logger *logger.Logger) (*IntrospectionResponse, error) {
	var response struct {
		IntrospectionResponse
	}

	query := `query IntrospectionQuery {
		    __schema {
		      queryType { name }
		      mutationType { name }
		      types {
		        ...FullType
		      }
		    }
		  }

		  fragment FullType on __Type {
		    kind
		    name
		    description
		    fields(includeDeprecated: true) {
		      name
		    }
		  }`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	return &response.IntrospectionResponse, err
}
