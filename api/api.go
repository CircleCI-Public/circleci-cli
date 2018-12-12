package api

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"fmt"

	"github.com/CircleCI-Public/circleci-cli/client"
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

// Options wraps common requirements for API functions into a single struct.
type Options struct {
	Context context.Context
	Client  *client.Client
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
		Versions []OrbVersion
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
				Node   OrbWithData
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
			Node   OrbWithData
		}
		PageInfo struct {
			HasNextPage bool
		}
	}
}

// OrbConfigResponse wraps the GQL result for OrbQuery.
type OrbConfigResponse struct {
	OrbConfig struct {
		ConfigResponse
	}
}

// OrbsForListing is a container type for multiple orbs that includes the namespace and orbs for deserializing back into JSON.
type OrbsForListing struct {
	Orbs      []OrbWithData `json:"orbs"`
	Namespace string        `json:"namespace,omitempty"`
}

// OrbWithData wraps an orb with select fields for deserializing into JSON.
type OrbWithData struct {
	Name           string `json:"name"`
	HighestVersion string `json:"version"`
	Versions       []struct {
		Version string `json:"version"`
		Source  string `json:"source"`
	} `json:"versions"`

	UsageStats struct {
		Last30DaysBuildCount        int
		Last30DaysProjectCount      int
		Last30DaysOrganizationCount int
	} `json:"-"`

	// These fields are printing manually when --details flag is added so hidden from JSON output.
	Commands  map[string]OrbElement `json:"-"`
	Jobs      map[string]OrbElement `json:"-"`
	Executors map[string]OrbElement `json:"-"`
}

// OrbElementParameter represents the yaml-unmarshled contents of
// a parameter for a command/job/executor
type OrbElementParameter struct {
	Description string      `json:"-"`
	Type        string      `json:"-"`
	Default     interface{} `json:"-"`
}

// RealOrbElement represents the yaml-unmarshled contents of
// a named element under a command/job/executor
type RealOrbElement struct {
	Description string                         `json:"-"`
	Parameters  map[string]OrbElementParameter `json:"-"`
}

// OrbElement implements RealOrbElement interface and allows us to deserialize by hand.
type OrbElement RealOrbElement

// UnmarshalYAML method allows OrbElement to be a string or a map.
// For now, don't even try to dereference the string, just return what is essentially
// an empty OrbElement (no description or parameters)
func (orbElement *OrbElement) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
	if err == nil {
		*orbElement = OrbElement{}
		return nil
	}

	var oe RealOrbElement
	err = unmarshal(&oe)
	if err == nil {
		*orbElement = OrbElement(oe)

		return nil
	}
	return nil
}

// Orb is a struct for containing the yaml-unmarshaled contents of an orb
type Orb struct {
	ID        string
	Name      string
	Namespace string
	CreatedAt string

	Source         string
	HighestVersion string `json:"version"`

	UsageStats struct {
		Last30DaysBuildCount        int
		Last30DaysProjectCount      int
		Last30DaysOrganizationCount int
	}

	Commands  map[string]OrbElement
	Jobs      map[string]OrbElement
	Executors map[string]OrbElement
	Versions  []OrbVersion
}

// OrbVersion wraps the GQL result used by OrbSource and OrbInfo
type OrbVersion struct {
	ID        string
	Version   string
	Orb       Orb
	Source    string
	CreatedAt string
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
func WhoamiQuery(opts Options) (*WhoamiResponse, error) {
	response := WhoamiResponse{}
	query := `query { me { name } }`

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	err = opts.Client.Run(opts.Context, request, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// ConfigQuery calls the GQL API to validate and process config
func ConfigQuery(opts Options, configPath string) (*ConfigResponse, error) {
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
	request.Header.Set("Authorization", opts.Client.Token)

	err = opts.Client.Run(opts.Context, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}

	if len(response.BuildConfig.ConfigResponse.Errors) > 0 {
		return nil, &response.BuildConfig.ConfigResponse.Errors
	}

	return &response.BuildConfig.ConfigResponse, nil
}

// OrbQuery validated and processes an orb.
func OrbQuery(opts Options, configPath string) (*ConfigResponse, error) {
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
	request.Header.Set("Authorization", opts.Client.Token)

	err = opts.Client.Run(opts.Context, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}

	if len(response.OrbConfig.ConfigResponse.Errors) > 0 {
		return nil, response.OrbConfig.ConfigResponse.Errors
	}

	return &response.OrbConfig.ConfigResponse, nil
}

// OrbPublishByID publishes a new version of an orb by id
func OrbPublishByID(opts Options, configPath string, orbID string, orbVersion string) (*Orb, error) {
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

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	request.Var("config", config)
	request.Var("orbId", orbID)
	request.Var("version", orbVersion)

	err = opts.Client.Run(opts.Context, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to publish orb")
	}

	if len(response.PublishOrb.Errors) > 0 {
		return nil, response.PublishOrb.Errors
	}

	return &response.PublishOrb.Orb, nil
}

// OrbID fetches an orb returning the ID
func OrbID(opts Options, namespace string, orb string) (*OrbIDResponse, error) {
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

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("namespace", namespace)

	err = opts.Client.Run(opts.Context, request, &response)

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

func createNamespaceWithOwnerID(opts Options, name string, ownerID string) (*CreateNamespaceResponse, error) {
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

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("organizationId", ownerID)

	err = opts.Client.Run(opts.Context, request, &response)

	if len(response.CreateNamespace.Errors) > 0 {
		return nil, response.CreateNamespace.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func getOrganization(opts Options, organizationName string, organizationVcs string) (*GetOrganizationResponse, error) {
	var response GetOrganizationResponse

	query := `query($organizationName: String!, $organizationVcs: VCSType!) {
				organization(
					name: $organizationName
					vcsType: $organizationVcs
				) {
					id
				}
			}`

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	request.Var("organizationName", organizationName)
	request.Var("organizationVcs", organizationVcs)

	err = opts.Client.Run(opts.Context, request, &response)

	if err != nil {
		return nil, errors.Wrapf(err, "Unable to find organization %s of vcs-type %s", organizationName, organizationVcs)
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
func CreateNamespace(opts Options, name string, organizationName string, organizationVcs string) (*CreateNamespaceResponse, error) {
	getOrgResponse, getOrgError := getOrganization(opts, organizationName, organizationVcs)

	if getOrgError != nil {
		return nil, errors.Wrap(organizationNotFound(organizationName, organizationVcs), getOrgError.Error())
	}

	createNSResponse, createNSError := createNamespaceWithOwnerID(opts, name, getOrgResponse.Organization.ID)

	if createNSError != nil {
		return nil, createNSError
	}

	return createNSResponse, nil
}

func getNamespace(opts Options, name string) (*GetNamespaceResponse, error) {
	var response GetNamespaceResponse

	query := `
				query($name: String!) {
					registryNamespace(
						name: $name
					){
						id
					}
			 }`

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)

	if err = opts.Client.Run(opts.Context, request, &response); err != nil {
		return nil, errors.Wrapf(err, "failed to load namespace '%s'", err)
	}

	if response.RegistryNamespace.ID == "" {
		return nil, namespaceNotFound(name)
	}

	return &response, nil
}

func createOrbWithNsID(opts Options, name string, namespaceID string) (*CreateOrbResponse, error) {
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

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("registryNamespaceId", namespaceID)

	err = opts.Client.Run(opts.Context, request, &response)

	if len(response.CreateOrb.Errors) > 0 {
		return nil, response.CreateOrb.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateOrb creates (reserves) an orb within a namespace
func CreateOrb(opts Options, namespace string, name string) (*CreateOrbResponse, error) {
	response, err := getNamespace(opts, namespace)
	if err != nil {
		return nil, err
	}

	return createOrbWithNsID(opts, name, response.RegistryNamespace.ID)
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
func OrbIncrementVersion(opts Options, configPath string, namespace string, orb string, segment string) (*Orb, error) {
	// TODO(zzak): We can squash OrbID and OrbLatestVersion to a single query
	id, err := OrbID(opts, namespace, orb)
	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(opts, namespace, orb)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	response, err := OrbPublishByID(opts, configPath, id.Orb.ID, v2)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// OrbLatestVersion finds the latest published version of an orb and returns it.
// If it doesn't find a version, it will return 0.0.0 for the orb's version
func OrbLatestVersion(opts Options, namespace string, orb string) (string, error) {
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

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return "", err
	}
	request.Var("name", name)

	err = opts.Client.Run(opts.Context, request, &response)
	if err != nil {
		return "", err
	}

	if len(response.Orb.Versions) != 1 {
		return "0.0.0", nil
	}

	return response.Orb.Versions[0].Version, nil
}

// OrbPromote takes an orb and a development version and increments a semantic release with the given segment.
func OrbPromote(opts Options, namespace string, orb string, label string, segment string) (*Orb, error) {
	// TODO(zzak): We can squash OrbID and OrbLatestVersion to a single query
	id, err := OrbID(opts, namespace, orb)

	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(opts, namespace, orb)
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

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}
	request.Var("orbId", id.Orb.ID)
	request.Var("devVersion", label)
	request.Var("semanticVersion", v2)

	err = opts.Client.Run(opts.Context, request, &response)

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
func OrbSource(opts Options, orbRef string) (string, error) {
	if err := references.IsOrbRefWithOptionalVersion(orbRef); err != nil {
		return "", err
	}

	ref := orbVersionRef(orbRef)

	var response struct {
		OrbVersion OrbVersion
	}

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

	err := opts.Client.Run(opts.Context, request, &response)
	if err != nil {
		return "", err
	}

	if response.OrbVersion.ID == "" {
		return "", fmt.Errorf("no Orb '%s' was found; please check that the Orb reference is correct", orbRef)
	}

	return response.OrbVersion.Source, nil
}

// OrbInfo gets the meta-data of an orb
func OrbInfo(opts Options, orbRef string) (*OrbVersion, error) {
	if err := references.IsOrbRefWithOptionalVersion(orbRef); err != nil {
		return nil, err
	}

	ref := orbVersionRef(orbRef)

	var response struct {
		OrbVersion OrbVersion
	}

	query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb {
                                    id
                                    createdAt
                                    name
	                            usageStats {
		                        last30DaysBuildCount,
		                        last30DaysProjectCount,
		                        last30DaysOrganizationCount
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

	err := opts.Client.Run(opts.Context, request, &response)
	if err != nil {
		return nil, err
	}

	if response.OrbVersion.ID == "" {
		return nil, fmt.Errorf("no Orb '%s' was found; please check that the Orb reference is correct", orbRef)
	}

	if len(response.OrbVersion.Orb.Versions) > 0 {
		v := response.OrbVersion.Orb.Versions[0]

		response.OrbVersion.Orb.HighestVersion = v.Version
	} else {
		response.OrbVersion.Orb.HighestVersion = "Not published"
	}

	// Parse the orb source to get its commands, executors and jobs
	err = yaml.Unmarshal([]byte(response.OrbVersion.Source), &response.OrbVersion.Orb)
	if err != nil {
		return nil, errors.Wrapf(err, "Corrupt Orb %s %s", response.OrbVersion.Orb.Name, response.OrbVersion.Version)
	}

	return &response.OrbVersion, nil
}

// ListOrbs queries the API to find all orbs.
// Returns a collection of Orb objects containing their relevant data.
func ListOrbs(opts Options, uncertified bool) (*OrbsForListing, error) {
	l := log.New(os.Stderr, "", 0)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    usageStats {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
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

	var orbs OrbsForListing

	var result OrbListResponse
	currentCursor := ""

	for {
		request := client.NewUnauthorizedRequest(query)
		request.Var("after", currentCursor)
		request.Var("certifiedOnly", !uncertified)

		err := opts.Client.Run(opts.Context, request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

	Orbs:
		for i := range result.Orbs.Edges {
			edge := result.Orbs.Edges[i]
			currentCursor = edge.Cursor

			if len(edge.Node.Versions) > 0 {

				v := edge.Node.Versions[0]

				edge.Node.HighestVersion = v.Version

				err := yaml.Unmarshal([]byte(edge.Node.Versions[0].Source), &edge.Node)

				if err != nil {
					l.Printf(errors.Wrapf(err, "Corrupt Orb %s %s", edge.Node.Name, v.Version).Error())
					continue Orbs
				}

				orbs.Orbs = append(orbs.Orbs, edge.Node)
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
// Returns a collection of Orb objects containing their relevant data.
func ListNamespaceOrbs(opts Options, namespace string) (*OrbsForListing, error) {
	l := log.New(os.Stderr, "", 0)

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
	                                usageStats {
		                           last30DaysBuildCount,
		                           last30DaysProjectCount,
		                           last30DaysOrganizationCount
	                               }
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
	var orbs OrbsForListing
	var result NamespaceOrbResponse
	currentCursor := ""

	for {
		request := client.NewUnauthorizedRequest(query)
		request.Var("after", currentCursor)
		request.Var("namespace", namespace)
		orbs.Namespace = namespace

		err := opts.Client.Run(opts.Context, request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

	NamespaceOrbs:
		for i := range result.RegistryNamespace.Orbs.Edges {
			edge := result.RegistryNamespace.Orbs.Edges[i]
			currentCursor = edge.Cursor

			if len(edge.Node.Versions) > 0 {
				v := edge.Node.Versions[0]

				edge.Node.HighestVersion = v.Version

				err := yaml.Unmarshal([]byte(edge.Node.Versions[0].Source), &edge.Node)
				if err != nil {
					l.Printf(errors.Wrapf(err, "Corrupt Orb %s %s", edge.Node.Name, v.Version).Error())
					continue NamespaceOrbs
				}
			} else {
				edge.Node.HighestVersion = "Not published"
			}

			orbs.Orbs = append(orbs.Orbs, edge.Node)
		}

		if !result.RegistryNamespace.Orbs.PageInfo.HasNextPage {
			break
		}
	}

	return &orbs, nil
}

// IntrospectionQuery makes a query on the API asking for bits of the schema
// This query isn't intended to get the entire schema, there are better tools for that.
func IntrospectionQuery(opts Options) (*IntrospectionResponse, error) {
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

	request, err := client.NewAuthorizedRequest(query, opts.Client.Token)
	if err != nil {
		return nil, err
	}

	err = opts.Client.Run(opts.Context, request, &response)

	return &response, err
}
