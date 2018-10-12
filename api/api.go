package api

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"strings"

	"fmt"

	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/CircleCI-Public/circleci-cli/settings"
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
	Data struct {
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
	Data struct {
		BuildConfig struct {
			ConfigResponse
		}
	}
}

// The OrbPublishResponse type matches the data shape of the GQL response for
// publishing an orb.
type OrbPublishResponse struct {
	Data struct {
		PublishOrb struct {
			Orb Orb

			Errors GQLErrorsCollection
		}
	}
}

// The OrbPromoteResponse type matches the data shape of the GQL response for
// promoting an orb.
type OrbPromoteResponse struct {
	Data struct {
		PromoteOrb struct {
			Orb Orb

			Errors GQLErrorsCollection
		}
	}
}

// OrbLatestVersionResponse wraps the GQL result of fetching an Orb and latest version
type OrbLatestVersionResponse struct {
	Data struct {
		Orb struct {
			Versions []struct {
				Version string
			}
		}
	}
}

// OrbIDResponse matches the GQL response for fetching an Orb and ID
type OrbIDResponse struct {
	Data struct {
		Orb struct {
			ID string
		}
		RegistryNamespace struct {
			ID string
		}
	}
}

// CreateNamespaceResponse type matches the data shape of the GQL response for
// creating a namespace
type CreateNamespaceResponse struct {
	Data struct {
		CreateNamespace struct {
			Namespace struct {
				CreatedAt string
				ID        string
			}

			Errors GQLErrorsCollection
		}
	}

	Errors GQLErrorsCollection
}

// GetOrganizationResponse type wraps the GQL response for fetching an organization and ID.
type GetOrganizationResponse struct {
	Data struct {
		Organization struct {
			ID string
		}
	}

	Errors GQLErrorsCollection
}

// WhoamiResponse type matches the data shape of the GQL response for the current user
type WhoamiResponse struct {
	Data struct {
		Me struct {
			Name string
		}
	}

	Errors GQLErrorsCollection
}

// GetNamespaceResponse type wraps the GQL response for fetching a namespace
type GetNamespaceResponse struct {
	Data struct {
		RegistryNamespace struct {
			ID string
		}
	}

	Errors GQLErrorsCollection
}

// CreateOrbResponse type matches the data shape of the GQL response for
// creating an orb
type CreateOrbResponse struct {
	Data struct {
		CreateOrb struct {
			Orb    Orb
			Errors GQLErrorsCollection
		}
	}

	Errors GQLErrorsCollection
}

// NamespaceOrbResponse type matches the result from GQL.
// So that we can use mapstructure to convert from nested maps to a strongly typed struct.
type NamespaceOrbResponse struct {
	Data struct {
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
}

// OrbListResponse type matches the result from GQL.
// So that we can use mapstructure to convert from nested maps to a strongly typed struct.
type OrbListResponse struct {
	Data struct {
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
}

// OrbSourceResponse wraps the GQL result used by OrbSource
type OrbSourceResponse struct {
	Data struct {
		OrbVersion struct {
			ID      string
			Version string
			Orb     struct {
				ID string
			}
			Source string
		}
	}
}

// OrbConfigResponse wraps the GQL result for OrbQuery.
type OrbConfigResponse struct {
	Data struct {
		OrbConfig struct {
			ConfigResponse
		}
	}
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
func WhoamiQuery(ctx context.Context, cfg *settings.Config) (*WhoamiResponse, error) {
	response := WhoamiResponse{}
	query := `query { me { name } }`

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	err = cfg.Client.Run(ctx, request, &response)

	if err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		return nil, response.Errors
	}

	return &response, nil
}

func buildAndOrbQuery(ctx context.Context, cfg *settings.Config, configPath string, response interface{}, query string) error {
	config, err := loadYaml(configPath)
	if err != nil {
		return err
	}

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return err
	}
	request.Var("config", config)

	err = cfg.Client.Run(ctx, request, &response)

	if err != nil {
		return errors.Wrap(err, "Unable to validate config")
	}

	return nil
}

// ConfigQuery calls the GQL API to validate and process config
func ConfigQuery(ctx context.Context, cfg *settings.Config, configPath string) (*ConfigResponse, error) {
	var response BuildConfigResponse

	err := buildAndOrbQuery(ctx, cfg, configPath, &response, `
		query ValidateConfig ($config: String!) {
			buildConfig(configYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`)

	if err != nil {
		return nil, err
	}

	if len(response.Data.BuildConfig.ConfigResponse.Errors) > 0 {
		return nil, &response.Data.BuildConfig.ConfigResponse.Errors
	}

	return &response.Data.BuildConfig.ConfigResponse, nil
}

// OrbQuery validated and processes an orb.
func OrbQuery(ctx context.Context, cfg *settings.Config, configPath string) (*ConfigResponse, error) {
	var response OrbConfigResponse

	err := buildAndOrbQuery(ctx, cfg, configPath, &response, `
		query ValidateOrb ($config: String!) {
			orbConfig(orbYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`)

	if err != nil {
		return nil, err
	}

	if len(response.Data.OrbConfig.ConfigResponse.Errors) > 0 {
		return nil, response.Data.OrbConfig.ConfigResponse.Errors
	}

	return &response.Data.OrbConfig.ConfigResponse, nil
}

// OrbPublishByID publishes a new version of an orb by id
func OrbPublishByID(ctx context.Context, cfg *settings.Config,
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

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	request.Var("config", config)
	request.Var("orbId", orbID)
	request.Var("version", orbVersion)

	err = cfg.Client.Run(ctx, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to publish orb")
	}

	if len(response.Data.PublishOrb.Errors) > 0 {
		return nil, response.Data.PublishOrb.Errors
	}

	return &response.Data.PublishOrb.Orb, nil
}

// OrbID fetches an orb returning the ID
func OrbID(ctx context.Context, cfg *settings.Config, namespace string, orb string) (*OrbIDResponse, error) {
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

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("namespace", namespace)

	err = cfg.Client.Run(ctx, request, &response)

	// If there is an error, or the request was successful, return now.
	if err != nil || response.Data.Orb.ID != "" {
		return &response, err
	}

	// Otherwise, we want to generate a nice error message for the user.
	namespaceExists := response.Data.RegistryNamespace.ID != ""
	if !namespaceExists {
		return nil, namespaceNotFound(namespace)
	}

	return nil, fmt.Errorf("the '%s' orb does not exist in the '%s' namespace. Did you misspell the namespace or the orb name?", orb, namespace)
}

func createNamespaceWithOwnerID(ctx context.Context, cfg *settings.Config, name string, ownerID string) (*CreateNamespaceResponse, error) {
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

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("organizationId", ownerID)

	err = cfg.Client.Run(ctx, request, &response)

	if len(response.Data.CreateNamespace.Errors) > 0 {
		return nil, response.Data.CreateNamespace.Errors
	}

	if err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		return nil, response.Errors
	}

	return &response, nil
}

func getOrganization(ctx context.Context, cfg *settings.Config, organizationName string, organizationVcs string) (*GetOrganizationResponse, error) {
	var response GetOrganizationResponse

	query := `query($organizationName: String!, $organizationVcs: VCSType!) {
				organization(
					name: $organizationName
					vcsType: $organizationVcs
				) {
					id
				}
			}`

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	request.Var("organizationName", organizationName)
	request.Var("organizationVcs", organizationVcs)

	err = cfg.Client.Run(ctx, request, &response)

	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Unable to find organization %s of vcs-type %s", organizationName, organizationVcs))
	}

	if len(response.Errors) > 0 {
		return nil, response.Errors
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
func CreateNamespace(ctx context.Context, cfg *settings.Config, name string, organizationName string, organizationVcs string) (*CreateNamespaceResponse, error) {
	getOrgResponse, getOrgError := getOrganization(ctx, cfg, organizationName, organizationVcs)

	if getOrgError != nil {
		return nil, errors.Wrap(organizationNotFound(organizationName, organizationVcs), getOrgError.Error())
	}

	createNSResponse, createNSError := createNamespaceWithOwnerID(ctx, cfg, name, getOrgResponse.Data.Organization.ID)

	if createNSError != nil {
		return nil, createNSError
	}

	return createNSResponse, nil
}

func getNamespace(ctx context.Context, cfg *settings.Config, name string) (*GetNamespaceResponse, error) {
	var response GetNamespaceResponse

	query := `
				query($name: String!) {
					registryNamespace(
						name: $name
					){
						id
					}
			 }`
	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)

	if err := cfg.Client.Run(ctx, request, &response); err != nil {
		return nil, errors.Wrapf(err, "failed to load namespace '%s'", err)
	}

	if response.Data.RegistryNamespace.ID == "" {
		return nil, namespaceNotFound(name)
	}

	if len(response.Errors) > 0 {
		return nil, response.Errors
	}

	return &response, nil
}

func createOrbWithNsID(ctx context.Context, cfg *settings.Config, name string, namespaceID string) (*CreateOrbResponse, error) {
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

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	request.Var("name", name)
	request.Var("registryNamespaceId", namespaceID)

	err = cfg.Client.Run(ctx, request, &response)

	if len(response.Data.CreateOrb.Errors) > 0 {
		return nil, response.Data.CreateOrb.Errors
	}

	if len(response.Errors) > 0 {
		return nil, response.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateOrb creates (reserves) an orb within a namespace
func CreateOrb(ctx context.Context, cfg *settings.Config, namespace string, name string) (*CreateOrbResponse, error) {
	response, err := getNamespace(ctx, cfg, namespace)
	if err != nil {
		return nil, err
	}

	return createOrbWithNsID(ctx, cfg, name, response.Data.RegistryNamespace.ID)
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
func OrbIncrementVersion(ctx context.Context, cfg *settings.Config, configPath string, namespace string, orb string, segment string) (*Orb, error) {
	id, err := OrbID(ctx, cfg, namespace, orb)
	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(ctx, cfg, namespace, orb)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	response, err := OrbPublishByID(ctx, cfg, configPath, id.Data.Orb.ID, v2)
	if err != nil {
		return nil, err
	}

	cfg.Logger.Debug("Bumped %s/%s#%s from %s by %s to %s\n.", namespace, orb, id.Data.Orb.ID, v, segment, v2)

	return response, nil
}

// OrbLatestVersion finds the latest published version of an orb and returns it.
// If it doesn't find a version, it will return 0.0.0 for the orb's version
func OrbLatestVersion(ctx context.Context, cfg *settings.Config, namespace string, orb string) (string, error) {
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

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return "", err
	}
	request.Var("name", name)

	err = cfg.Client.Run(ctx, request, &response)

	if err != nil {
		return "", err
	}

	if len(response.Data.Orb.Versions) != 1 {
		return "0.0.0", nil
	}

	return response.Data.Orb.Versions[0].Version, nil
}

// OrbPromote takes an orb and a development version and increments a semantic release with the given segment.
func OrbPromote(ctx context.Context, cfg *settings.Config, namespace string, orb string, label string, segment string) (*Orb, error) {
	id, err := OrbID(ctx, cfg, namespace, orb)

	if err != nil {
		return nil, err
	}

	v, err := OrbLatestVersion(ctx, cfg, namespace, orb)
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

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}
	request.Var("orbId", id.Data.Orb.ID)
	request.Var("devVersion", label)
	request.Var("semanticVersion", v2)

	err = cfg.Client.Run(ctx, request, &response)

	if len(response.Data.PromoteOrb.Errors) > 0 {
		return nil, response.Data.PromoteOrb.Errors
	}

	if err != nil {
		return nil, errors.Wrap(err, "Unable to promote orb")
	}

	return &response.Data.PromoteOrb.Orb, nil
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
func OrbSource(ctx context.Context, cfg *settings.Config, orbRef string) (string, error) {

	if err := references.IsOrbRefWithOptionalVersion(orbRef); err != nil {
		return "", err
	}

	ref := orbVersionRef(orbRef)

	var response OrbSourceResponse

	query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return "", err
	}
	request.Var("orbVersionRef", ref)

	err = cfg.Client.Run(ctx, request, &response)

	if err != nil {
		return "", err
	}

	if response.Data.OrbVersion.ID == "" {
		return "", fmt.Errorf("no Orb '%s' was found; please check that the Orb reference is correct", orbRef)
	}

	return response.Data.OrbVersion.Source, nil
}

// ListOrbs queries the API to find all orbs.
// Returns a collection of Orb objects containing their relevant data. Logs
// request and parse errors to the supplied logger.
func ListOrbs(ctx context.Context, cfg *settings.Config, uncertified bool) (*OrbCollection, error) {
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
		request, err := cfg.Client.NewAuthorizedRequest(query)
		if err != nil {
			return nil, err
		}
		request.Var("after", currentCursor)
		request.Var("certifiedOnly", !uncertified)

		err = cfg.Client.Run(ctx, request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

	Orbs:
		for i := range result.Data.Orbs.Edges {
			edge := result.Data.Orbs.Edges[i]
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
					cfg.Logger.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
					continue Orbs
				}
				orbs.Orbs = append(orbs.Orbs, o)
			}
		}

		if !result.Data.Orbs.PageInfo.HasNextPage {
			break
		}
	}
	return &orbs, nil
}

// ListNamespaceOrbs queries the API to find all orbs belonging to the given
// namespace.
// Returns a collection of Orb objects containing their relevant data. Logs
// request and parse errors to the supplied logger.
func ListNamespaceOrbs(ctx context.Context, cfg *settings.Config, namespace string) (*OrbCollection, error) {
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
		request, err := cfg.Client.NewAuthorizedRequest(query)
		if err != nil {
			return nil, err
		}
		request.Var("after", currentCursor)
		request.Var("namespace", namespace)
		orbs.Namespace = namespace

		err = cfg.Client.Run(ctx, request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

	NamespaceOrbs:
		for i := range result.Data.RegistryNamespace.Orbs.Edges {
			edge := result.Data.RegistryNamespace.Orbs.Edges[i]
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
					cfg.Logger.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
					continue NamespaceOrbs
				}
			} else {
				o.HighestVersion = "Not published"
				o.Versions = []OrbVersion{}
			}
			orbs.Orbs = append(orbs.Orbs, o)
		}

		if !result.Data.RegistryNamespace.Orbs.PageInfo.HasNextPage {
			break
		}
	}

	return &orbs, nil
}

// IntrospectionQuery makes a query on the API asking for bits of the schema
// This query isn't intended to get the entire schema, there are better tools for that.
func IntrospectionQuery(ctx context.Context, cfg *settings.Config) (*IntrospectionResponse, error) {
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

	request, err := cfg.Client.NewAuthorizedRequest(query)
	if err != nil {
		return nil, err
	}

	err = cfg.Client.Run(ctx, request, &response)

	return &response, err
}
