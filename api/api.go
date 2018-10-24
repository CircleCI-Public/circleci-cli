package api

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"fmt"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// GQLErrorsCollection is a slice of errors returned by the GraphQL server.
// Each error is made up of a GQLResponseError type.
type GQLErrorsCollection []GQLResponseError

// Error turns a GQLErrorsCollection into an acceptable error string that can be printed to the user.
func (errs GQLErrorsCollection) Error() string {
	messages := []string{}

	for i := range errs {
		messages = append(messages, errs[i].Message)
	}

	return strings.Join(messages, "\n")
}

// GQLResponseError is a mapping of the data returned by the GraphQL server of key-value pairs.
// Typically used with the structure "Message: string", but other response errors provide additional fields.
type GQLResponseError struct {
	Message       string
	Value         string
	AllowedValues []string
	EnumType      string
	Type          string
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

	Errors GQLErrorsCollection
}

// BuildConfigResponse wraps the GQL result of the ConfigQuery
type BuildConfigResponse struct {
	BuildConfig struct {
		ConfigResponse
	}
}

// The OrbPublishResponse type matches the data shape of the GQL response for
// publishing an orb.
type OrbPublishResponse struct {
	PublishOrb struct {
		Orb Orb

		Errors GQLErrorsCollection
	}
}

// The OrbPromoteResponse type matches the data shape of the GQL response for
// promoting an orb.
type OrbPromoteResponse struct {
	PromoteOrb struct {
		Orb Orb

		Errors GQLErrorsCollection
	}
}

// OrbLatestVersionResponse wraps the GQL result of fetching an Orb and latest version
type OrbLatestVersionResponse struct {
	Orb struct {
		Versions []struct {
			Version string
		}
	}
}

// OrbIDResponse matches the GQL response for fetching an Orb and ID
type OrbIDResponse struct {
	Orb struct {
		ID string
	}
	RegistryNamespace struct {
		ID string
	}
}

// CreateNamespaceResponse type matches the data shape of the GQL response for
// creating a namespace
type CreateNamespaceResponse struct {
	CreateNamespace struct {
		Namespace struct {
			CreatedAt string
			ID        string
		}

		Errors GQLErrorsCollection
	}
}

// GetOrganizationResponse type wraps the GQL response for fetching an organization and ID.
type GetOrganizationResponse struct {
	Organization struct {
		ID string
	}
}

// WhoamiResponse type matches the data shape of the GQL response for the current user
type WhoamiResponse struct {
	Me struct {
		Name string
	}
}

// GetNamespaceResponse type wraps the GQL response for fetching a namespace
type GetNamespaceResponse struct {
	RegistryNamespace struct {
		ID string
	}
}

// CreateOrbResponse type matches the data shape of the GQL response for
// creating an orb
type CreateOrbResponse struct {
	CreateOrb struct {
		Orb    Orb
		Errors GQLErrorsCollection
	}
}

// NamespaceOrbResponse type matches the result from GQL.
// So that we can use mapstructure to convert from nested maps to a strongly typed struct.
type NamespaceOrbResponse struct {
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

// OrbListResponse type matches the result from GQL.
// So that we can use mapstructure to convert from nested maps to a strongly typed struct.
type OrbListResponse struct {
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

// OrbVersionResponse wraps the GQL result used by OrbSource and OrbInfo
type OrbVersionResponse struct {
	OrbVersion struct {
		ID      string
		Version string
		Orb     struct {
			ID        string
			CreatedAt string
			Name      string
			Namespace struct {
				Name string
			}
			Versions []struct {
				Version   string
				CreatedAt string
			}
		}
		Source    string
		CreatedAt string
	}
}

// OrbConfigResponse wraps the GQL result for OrbQuery.
type OrbConfigResponse struct {
	OrbConfig struct {
		ConfigResponse
	}
}

// OrbCollection is a container type for multiple orbs to share formatting
// functions on them.
type OrbCollection struct {
	Orbs      []Orb  `json:"orbs"`
	Namespace string `json:"namespace,omitempty"`
}

// OrbVersion represents a single orb version and its source
type OrbVersion struct {
	Version string `json:"version"`
	Source  string `json:"source"`
}

// OrbElementParameter represents the yaml-unmarshled contents of
// a parameter for a command/job/executor
type OrbElementParameter struct {
	Description string      `json:"-"`
	Type        string      `json:"-"`
	Default     interface{} `json:"-"`
}

// OrbElement represents the yaml-unmarshled contents of
// a named element under a command/job/executor
type OrbElement struct {
	Description string                         `json:"-"`
	Parameters  map[string]OrbElementParameter `json:"-"`
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
	HighestVersion string                `json:"version"`
	Version        string                `json:"-"`
	Commands       map[string]OrbElement `json:"-"`
	Jobs           map[string]OrbElement `json:"-"`
	Executors      map[string]OrbElement `json:"-"`
	Versions       []OrbVersion          `json:"versions"`
}

// #nosec
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
func WhoamiQuery(ctx context.Context, log *logger.Logger, cl *client.Client) (*WhoamiResponse, error) {
	response := WhoamiResponse{}
	query := `query { me { name } }`

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	err = cl.Run(ctx, log, request, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// ConfigQuery calls the GQL API to validate and process config
func ConfigQuery(ctx context.Context, log *logger.Logger, cl *client.Client, configPath string) (*ConfigResponse, error) {
	var response BuildConfigResponse

	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	query := `
		query ValidateConfig ($config: String!) {
			buildConfig(configYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`

	request := client.NewUnauthorizedRequest(query)
	request.Var("config", config)
	request.Header.Set("Authorization", cl.Token)

	err = cl.Run(ctx, log, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}

	if len(response.BuildConfig.ConfigResponse.Errors) > 0 {
		return nil, &response.BuildConfig.ConfigResponse.Errors
	}

	return &response.BuildConfig.ConfigResponse, nil
}

// OrbQuery validated and processes an orb.
func OrbQuery(ctx context.Context, log *logger.Logger, cl *client.Client, configPath string) (*ConfigResponse, error) {
	var response OrbConfigResponse

	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	query := `
		query ValidateOrb ($config: String!) {
			orbConfig(orbYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`

	request := client.NewUnauthorizedRequest(query)
	request.Var("config", config)
	request.Header.Set("Authorization", cl.Token)

	err = cl.Run(ctx, log, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}

	if len(response.OrbConfig.ConfigResponse.Errors) > 0 {
		return nil, response.OrbConfig.ConfigResponse.Errors
	}

	return &response.OrbConfig.ConfigResponse, nil
}

// OrbPublishByID publishes a new version of an orb by id
func OrbPublishByID(ctx context.Context, log *logger.Logger, cl *client.Client,
	configPath string, orbID string, orbVersion string) (*Orb, error) {

	var response OrbPublishResponse

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

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	request.Var("config", config)
	request.Var("orbId", orbID)
	request.Var("version", orbVersion)

	err = cl.Run(ctx, log, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to publish orb")
	}

	if len(response.PublishOrb.Errors) > 0 {
		return nil, response.PublishOrb.Errors
	}

	return &response.PublishOrb.Orb, nil
}

// OrbID fetches an orb returning the ID
func OrbID(ctx context.Context, log *logger.Logger, cl *client.Client, namespace string, orb string) (*OrbIDResponse, error) {
	name := namespace + "/" + orb

	var response OrbIDResponse

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

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("namespace", namespace)

	err = cl.Run(ctx, log, request, &response)

	// If there is an error, or the request was successful, return now.
	if err != nil || response.Orb.ID != "" {
		return &response, err
	}

	// Otherwise, we want to generate a nice error message for the user.
	namespaceExists := response.RegistryNamespace.ID != ""
	if !namespaceExists {
		return nil, namespaceNotFound(namespace)
	}

	return nil, fmt.Errorf("the '%s' orb does not exist in the '%s' namespace. Did you misspell the namespace or the orb name?", orb, namespace)
}

func createNamespaceWithOwnerID(ctx context.Context, log *logger.Logger, cl *client.Client, name string, ownerID string) (*CreateNamespaceResponse, error) {
	var response CreateNamespaceResponse

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

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("organizationId", ownerID)

	err = cl.Run(ctx, log, request, &response)

	if len(response.CreateNamespace.Errors) > 0 {
		return nil, response.CreateNamespace.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func getOrganization(ctx context.Context, log *logger.Logger, cl *client.Client, organizationName string, organizationVcs string) (*GetOrganizationResponse, error) {
	var response GetOrganizationResponse

	query := `query($organizationName: String!, $organizationVcs: VCSType!) {
				organization(
					name: $organizationName
					vcsType: $organizationVcs
				) {
					id
				}
			}`

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	request.Var("organizationName", organizationName)
	request.Var("organizationVcs", organizationVcs)

	err = cl.Run(ctx, log, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Unable to find organization %s of vcs-type %s", organizationName, organizationVcs))
	}

	return &response, nil
}

func namespaceNotFound(name string) error {
	return fmt.Errorf("the namespace '%s' does not exist. Did you misspell the namespace, or maybe you meant to create the namespace first?", name)
}

func organizationNotFound(name string, vcs string) error {
	return fmt.Errorf("the organization '%s' under '%s' VCS-type does not exist. Did you misspell the organization or VCS?", name, vcs)
}

// CreateNamespace creates (reserves) a namespace for an organization
func CreateNamespace(ctx context.Context, log *logger.Logger, cl *client.Client, name string, organizationName string, organizationVcs string) (*CreateNamespaceResponse, error) {
	getOrgResponse, getOrgError := getOrganization(ctx, log, cl, organizationName, organizationVcs)

	if getOrgError != nil {
		return nil, errors.Wrap(organizationNotFound(organizationName, organizationVcs), getOrgError.Error())
	}

	createNSResponse, createNSError := createNamespaceWithOwnerID(ctx, log, cl, name, getOrgResponse.Organization.ID)

	if createNSError != nil {
		return nil, createNSError
	}

	return createNSResponse, nil
}

func getNamespace(ctx context.Context, log *logger.Logger, cl *client.Client, name string) (*GetNamespaceResponse, error) {
	var response GetNamespaceResponse

	query := `
				query($name: String!) {
					registryNamespace(
						name: $name
					){
						id
					}
			 }`

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)

	if err = cl.Run(ctx, log, request, &response); err != nil {
		return nil, errors.Wrapf(err, "failed to load namespace '%s'", err)
	}

	if response.RegistryNamespace.ID == "" {
		return nil, namespaceNotFound(name)
	}

	return &response, nil
}

func createOrbWithNsID(ctx context.Context, log *logger.Logger, cl *client.Client, name string, namespaceID string) (*CreateOrbResponse, error) {
	var response CreateOrbResponse

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

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("registryNamespaceId", namespaceID)

	err = cl.Run(ctx, log, request, &response)

	if len(response.CreateOrb.Errors) > 0 {
		return nil, response.CreateOrb.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateOrb creates (reserves) an orb within a namespace
func CreateOrb(ctx context.Context, log *logger.Logger, cl *client.Client, namespace string, name string) (*CreateOrbResponse, error) {
	response, err := getNamespace(ctx, log, cl, namespace)
	if err != nil {
		return nil, err
	}

	return createOrbWithNsID(ctx, log, cl, name, response.RegistryNamespace.ID)
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
func OrbIncrementVersion(ctx context.Context, log *logger.Logger, cl *client.Client, configPath string, namespace string, orb string, segment string) (*Orb, error) {
	id, err := OrbID(ctx, log, cl, namespace, orb)
	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(ctx, log, cl, namespace, orb)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	response, err := OrbPublishByID(ctx, log, cl, configPath, id.Orb.ID, v2)
	if err != nil {
		return nil, err
	}

	log.Debug("Bumped %s/%s#%s from %s by %s to %s\n.", namespace, orb, id.Orb.ID, v, segment, v2)

	return response, nil
}

// OrbLatestVersion finds the latest published version of an orb and returns it.
// If it doesn't find a version, it will return 0.0.0 for the orb's version
func OrbLatestVersion(ctx context.Context, log *logger.Logger, cl *client.Client, namespace string, orb string) (string, error) {
	name := namespace + "/" + orb

	var response OrbLatestVersionResponse

	// This query returns versions sorted by semantic version
	query := `query($name: String!) {
			    orb(name: $name) {
			      versions(count: 1) {
				    version
			      }
			    }
		      }`

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return "", err
	}
	request.Var("name", name)

	err = cl.Run(ctx, log, request, &response)
	if err != nil {
		return "", err
	}

	if len(response.Orb.Versions) != 1 {
		return "0.0.0", nil
	}

	return response.Orb.Versions[0].Version, nil
}

// OrbPromote takes an orb and a development version and increments a semantic release with the given segment.
func OrbPromote(ctx context.Context, log *logger.Logger, cl *client.Client, namespace string, orb string, label string, segment string) (*Orb, error) {
	id, err := OrbID(ctx, log, cl, namespace, orb)

	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(ctx, log, cl, namespace, orb)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	var response OrbPromoteResponse

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

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}
	request.Var("orbId", id.Orb.ID)
	request.Var("devVersion", label)
	request.Var("semanticVersion", v2)

	err = cl.Run(ctx, log, request, &response)

	if len(response.PromoteOrb.Errors) > 0 {
		return nil, response.PromoteOrb.Errors
	}

	if err != nil {
		return nil, errors.Wrap(err, "Unable to promote orb")
	}

	return &response.PromoteOrb.Orb, nil
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

// OrbSource gets the source of an orb
func OrbSource(ctx context.Context, log *logger.Logger, cl *client.Client, orbRef string) (string, error) {
	if err := references.IsOrbRefWithOptionalVersion(orbRef); err != nil {
		return "", err
	}

	ref := orbVersionRef(orbRef)

	var response OrbVersionResponse

	query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`

	request := client.NewUnauthorizedRequest(query)
	request.Var("orbVersionRef", ref)

	err := cl.Run(ctx, log, request, &response)
	if err != nil {
		return "", err
	}

	if response.OrbVersion.ID == "" {
		return "", fmt.Errorf("no Orb '%s' was found; please check that the Orb reference is correct", orbRef)
	}

	return response.OrbVersion.Source, nil
}

// OrbInfo gets the meta-data of an orb
func OrbInfo(ctx context.Context, log *logger.Logger, cl *client.Client, orbRef string) (*OrbVersionResponse, error) {
	if err := references.IsOrbRefWithOptionalVersion(orbRef); err != nil {
		return nil, err
	}

	ref := orbVersionRef(orbRef)

	var response OrbVersionResponse

	query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb {
                                    id
                                    createdAt
                                    name
                                    namespace {
                                        name
                                    }
                                    versions {
                                        createdAt
                                        version
                                    }
                                }
                                source
                                createdAt
			    }
		      }`

	request := client.NewUnauthorizedRequest(query)
	request.Var("orbVersionRef", ref)

	err := cl.Run(ctx, log, request, &response)
	if err != nil {
		return nil, err
	}

	if response.OrbVersion.ID == "" {
		return nil, fmt.Errorf("no Orb '%s' was found; please check that the Orb reference is correct", orbRef)
	}

	return &response, nil
}

// ListOrbs queries the API to find all orbs.
// Returns a collection of Orb objects containing their relevant data. Logs
// request and parse errors to the supplied logger.
func ListOrbs(ctx context.Context, log *logger.Logger, cl *client.Client, uncertified bool) (*OrbCollection, error) {
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

	var result OrbListResponse
	currentCursor := ""

	for {
		request := client.NewUnauthorizedRequest(query)
		request.Var("after", currentCursor)
		request.Var("certifiedOnly", !uncertified)

		err := cl.Run(ctx, log, request, &result)
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
					log.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
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
func ListNamespaceOrbs(ctx context.Context, log *logger.Logger, cl *client.Client, namespace string) (*OrbCollection, error) {
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
	var result NamespaceOrbResponse
	currentCursor := ""

	for {
		request := client.NewUnauthorizedRequest(query)
		request.Var("after", currentCursor)
		request.Var("namespace", namespace)
		orbs.Namespace = namespace

		err := cl.Run(ctx, log, request, &result)
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
					log.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
					continue NamespaceOrbs
				}
			} else {
				o.HighestVersion = "Not published"
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
func IntrospectionQuery(ctx context.Context, log *logger.Logger, cl *client.Client) (*IntrospectionResponse, error) {
	var response IntrospectionResponse

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

	request, err := client.NewAuthorizedRequest(query, cl.Token)
	if err != nil {
		return nil, err
	}

	err = cl.Run(ctx, log, request, &response)

	return &response, err
}
