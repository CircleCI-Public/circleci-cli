package api

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"fmt"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// GQLResponseErrors is a slice of errors returned by the GraphQL server. Each
// error message is a key-value pair with the structure "Message: string"
type GQLResponseErrors struct {
	Errors []struct {
		Message string
	}
}

// IntrospectionResponse matches the result from making an introspection query
type IntrospectionResponse struct {
	Schema struct {
		MutationType struct {
			Name string
		}
		QueryType struct {
			Name string
		}
		Types []struct {
			Description string
			Fields      []struct {
				Name string
			}
			Kind string
			Name string
		}
	} `json:"__schema"`
}

// ConfigResponse is a structure that matches the result of the GQL
// query, so that we can use mapstructure to convert from
// nested maps to a strongly typed struct.
type ConfigResponse struct {
	Valid      bool
	SourceYaml string
	OutputYaml string

	GQLResponseErrors
}

// The OrbPublishResponse type matches the data shape of the GQL response for
// publishing an orb.
type OrbPublishResponse struct {
	Orb Orb
	GQLResponseErrors
}

// The OrbPromoteResponse type matches the data shape of the GQL response for
// promoting an orb.
type OrbPromoteResponse struct {
	Orb Orb
	GQLResponseErrors
}

// CreateNamespaceResponse type matches the data shape of the GQL response for
// creating a namespace
type CreateNamespaceResponse struct {
	Namespace struct {
		CreatedAt string
		ID        string
	}

	GQLResponseErrors
}

// WhoamiResponse type matches the data shape of the GQL response for the current user
type WhoamiResponse struct {
	Me struct {
		Name string
	}

	GQLResponseErrors
}

// CreateOrbResponse type matches the data shape of the GQL response for
// creating an orb
type CreateOrbResponse struct {
	Orb Orb
	GQLResponseErrors
}

// OrbCollection is a container type for multiple orbs to share formatting
// functions on them.
type OrbCollection struct {
	Orbs      []Orb  `json:"orbs"`
	Namespace string `json:"namespace,omitempty"`
}

// String returns a text representation of all Orbs, intended for
// direct human use rather than machine use.
func (orbCollection OrbCollection) String() string {
	var result string
	for _, o := range orbCollection.Orbs {
		result += (o.String())
	}
	return result
}

// OrbVersion represents a single orb version and its source
type OrbVersion struct {
	Version string `json:"version"`
	Source  string `json:"source"`
}

// Orb is a struct for containing the yaml-unmarshaled contents of an orb
type Orb struct {
	ID        string `json:"-"`
	Name      string `json:"name"`
	Namespace string `json:"-"`
	CreatedAt string `json:"-"`

	Source string `json:"-"`
	// Avoid "Version" since there is a "version" key in the orb source referring
	// to the orb schema version
	HighestVersion string              `json:"version"`
	Version        string              `json:"-"`
	Commands       map[string]struct{} `json:"-"`
	Jobs           map[string]struct{} `json:"-"`
	Executors      map[string]struct{} `json:"-"`
	Versions       []OrbVersion        `json:"versions"`
}

func addOrbElementsToBuffer(buf *bytes.Buffer, name string, elems map[string]struct{}) {
	var err error
	if len(elems) > 0 {
		_, err = buf.WriteString(fmt.Sprintf("  %s:\n", name))
		for key := range elems {
			_, err = buf.WriteString(fmt.Sprintf("    - %s\n", key))
		}
	}
	// This will never occur. The docs for bytes.Buffer.WriteString says err
	// will always be nil. The linter still expects this error to be checked.
	if err != nil {
		panic(err)
	}
}

// String returns a text representation of the Orb contents, intended for
// direct human use rather than machine use. This function will exclude orb
// source and orbs without any versions in its returned string.
func (orb Orb) String() string {
	var buffer bytes.Buffer

	_, err := buffer.WriteString(fmt.Sprintln(orb.Name, "("+orb.HighestVersion+")"))
	if err != nil {
		// The WriteString docstring says that it will never return an error
		panic(err)
	}
	addOrbElementsToBuffer(&buffer, "Commands", orb.Commands)
	addOrbElementsToBuffer(&buffer, "Jobs", orb.Jobs)
	addOrbElementsToBuffer(&buffer, "Executors", orb.Executors)

	return buffer.String()
}

// ToError returns all GraphQL errors for a single response concatenated, or
// nil.
func (response GQLResponseErrors) ToError() error {
	messages := []string{}

	for i := range response.Errors {
		messages = append(messages, response.Errors[i].Message)
	}

	return errors.New(strings.Join(messages, ": "))
}

// EnvEndpointHost pulls the endpoint and host values from viper
func EnvEndpointHost() (string, string) {
	return viper.GetString("endpoint"), viper.GetString("host")
}

// GraphQLServerAddress returns the full address to CircleCI GraphQL API server
func GraphQLServerAddress(endpoint string, host string) (string, error) {
	// 1. Parse the endpoint
	e, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.Wrapf(err, "Parsing endpoint '%s'", endpoint)
	}

	// 2. Parse the host
	h, err := url.Parse(host)
	if err != nil {
		return "", errors.Wrapf(err, "Parsing host '%s'", host)
	}
	if !h.IsAbs() {
		return h.String(), fmt.Errorf("Host (%s) must be absolute URL, including scheme", host)
	}

	// 3. Resolve the two URLs using host as the base
	// We use ResolveReference which has specific behavior we can rely for
	// older configurations which included the absolute path for the endpoint flag.
	//
	// https://golang.org/pkg/net/url/#URL.ResolveReference
	//
	// Specifically this function always returns the reference (endpoint) if provided an absolute URL.
	// This way we can safely introduce --host and merge the two.
	return h.ResolveReference(e).String(), err
}

// nolint: gosec
func loadYaml(path string) (string, error) {
	var err error
	var config []byte
	if path == "-" {
		config, err = ioutil.ReadAll(os.Stdin)
	} else {
		config, err = ioutil.ReadFile(path)
	}

	if err != nil {
		return "", errors.Wrapf(err, "Could not load config file at %s", path)
	}

	return string(config), nil
}

// WhoamiQuery returns the result of querying the `/me` endpoint of the API
func WhoamiQuery(ctx context.Context, logger *logger.Logger) (*WhoamiResponse, error) {
	response := WhoamiResponse{}
	query := `query { me { name } }`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func buildAndOrbQuery(ctx context.Context, logger *logger.Logger, configPath string, response interface{}, query string) error {
	config, err := loadYaml(configPath)
	if err != nil {
		return err
	}

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("config", config)
	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, response)

	if err != nil {
		return errors.Wrap(err, "Unable to validate config")
	}

	return nil
}

// ConfigQuery calls the GQL API to validate and process config
func ConfigQuery(ctx context.Context, logger *logger.Logger, configPath string) (*ConfigResponse, error) {
	var response struct {
		BuildConfig struct {
			ConfigResponse
		}
	}
	return &response.BuildConfig.ConfigResponse, buildAndOrbQuery(ctx, logger, configPath, &response, `
		query ValidateConfig ($config: String!) {
			buildConfig(configYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`)
}

// OrbQuery validated and processes an orb.
func OrbQuery(ctx context.Context, logger *logger.Logger, configPath string) (*ConfigResponse, error) {
	var response struct {
		OrbConfig struct {
			ConfigResponse
		}
	}

	return &response.OrbConfig.ConfigResponse, buildAndOrbQuery(ctx, logger, configPath, &response, `
		query ValidateOrb ($config: String!) {
			orbConfig(orbYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`)
}

// OrbPublishByID publishes a new version of an orb by id
func OrbPublishByID(ctx context.Context, logger *logger.Logger,
	configPath string, orbID string, orbVersion string) (*Orb, error) {

	var response struct {
		PublishOrb struct {
			OrbPublishResponse
		}
	}

	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	query := `
		mutation($config: String!, $orbId: UUID!, $version: String!) {
			publishOrb(
				orbId: $orbId,
				orbYaml: $config,
				version: $version
			) {
				orb {
					version
				}
				errors { message }
			}
		}
	`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("config", config)
	request.Var("orbId", orbID)
	request.Var("version", orbVersion)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return nil, err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to publish orb")
	}

	if len(response.PublishOrb.OrbPublishResponse.Errors) > 0 {
		return nil, response.PublishOrb.OrbPublishResponse.ToError()
	}

	return &response.PublishOrb.OrbPublishResponse.Orb, err
}

// OrbID fetches an orb returning the ID
func OrbID(ctx context.Context, logger *logger.Logger, namespace string, orb string) (string, error) {
	name := namespace + "/" + orb

	var response struct {
		Orb struct {
			ID string
		}
		RegistryNamespace struct {
			ID string
		}
	}

	query := `
	query ($name: String!, $namespace: String) {
		orb(name: $name) {
		  id
		}
		registryNamespace(name: $namespace) {
		  id
		}
	  }
	  `

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("name", name)
	request.Var("namespace", namespace)

	address, err := GraphQLServerAddress(EnvEndpointHost())
	if err != nil {
		return "", err
	}
	graphQLclient := client.NewClient(address, logger)

	err = graphQLclient.Run(ctx, request, &response)

	// If there is an error, or the request was successful, return now.
	if err != nil || response.Orb.ID != "" {
		return response.Orb.ID, err
	}

	// Otherwise, we want to generate a nice error message for the user.
	namespaceExists := response.RegistryNamespace.ID != ""
	if !namespaceExists {
		return "", namespaceNotFound(namespace)
	}

	return "", fmt.Errorf("the '%s' orb does not exist in the '%s' namespace. Did you misspell the namespace or the orb name?", orb, namespace)
}

func createNamespaceWithOwnerID(ctx context.Context, logger *logger.Logger, name string, ownerID string) (*CreateNamespaceResponse, error) {
	var response struct {
		CreateNamespace struct {
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
			var o Orb
			o.Name = edge.Node.Name
			if len(edge.Node.Versions) > 0 {
				v := edge.Node.Versions[0]

				// Parse the orb source to print its commands, executors and jobs
				o.HighestVersion = v.Version
				for _, v := range edge.Node.Versions {
					o.Versions = append(o.Versions, OrbVersion(v))
				}
				err := yaml.Unmarshal([]byte(edge.Node.Versions[0].Source), &o)
				if err != nil {
					logger.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
					continue NamespaceOrbs
				}
			} else {
				o.HighestVersion = "Unpublish"
				o.Versions = []OrbVersion{}
			}
			orbs.Orbs = append(orbs.Orbs, o)
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
