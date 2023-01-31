package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/pipeline"
	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type UpdateOrbCategorizationRequestType int

const (
	Add UpdateOrbCategorizationRequestType = iota
	Remove
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

// The OrbImportVersionResponse type matches the data shape of the GQL response for
// importing an orb version.
type OrbImportVersionResponse struct {
	ImportOrbVersion struct {
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

// The OrbSetOrbListStatusResponse type matches the data shape of the GQL response for
// setting the list status of an orb.
type OrbSetOrbListStatusResponse struct {
	SetOrbListStatus struct {
		Listed bool

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
		ID        string
		IsPrivate bool
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

// ImportNamespaceResponse type matches the data shape of the GQL response for
// importing a namespace
type ImportNamespaceResponse struct {
	ImportNamespace struct {
		Namespace struct {
			CreatedAt string
			ID        string
		}

		Errors GQLErrorsCollection
	}
}

type RenameNamespaceResponse struct {
	RenameNamespace struct {
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

// ImportOrbResponse type matches the data shape of the GQL response for
// creating an orb
type ImportOrbResponse struct {
	ImportOrb struct {
		Orb    Orb
		Errors GQLErrorsCollection
	}
}

// NamespaceOrbResponse type matches the result from GQL.
// So that we can use mapstructure to convert from nested maps to a strongly typed struct.
type NamespaceOrbResponse struct {
	RegistryNamespace struct {
		Name string
		ID   string
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

// NamespaceOrbVersionResponse type mat
type NamespaceOrbVersionResponse struct {
	RegistryNamespace struct {
		Name string
		ID   string
		Orbs struct {
			Edges []struct {
				Cursor string
				Node   Orb
			}
			PageInfo struct {
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

// OrbCategoryIDResponse matches the GQL response for fetching an Orb category's id
type OrbCategoryIDResponse struct {
	OrbCategoryByName struct {
		ID string
	}
}

// AddOrRemoveOrbCategorizationResponse type matches the data shape of the GQL response for
// adding or removing an orb categorization
type AddOrRemoveOrbCategorizationResponse map[string]AddOrRemoveOrbCategorizationData

type AddOrRemoveOrbCategorizationData struct {
	CategoryId string
	OrbId      string
	Errors     GQLErrorsCollection
}

// OrbCategoryListResponse type matches the result from GQL.
// So that we can use mapstructure to convert from nested maps to a strongly typed struct.
type OrbCategoryListResponse struct {
	OrbCategories struct {
		TotalCount int
		Edges      []struct {
			Cursor string
			Node   OrbCategory
		}
		PageInfo struct {
			HasNextPage bool
		}
	}
}

// OrbCategoriesForListing is a container type for multiple orb categories for deserializing back into JSON.
type OrbCategoriesForListing struct {
	OrbCategories []OrbCategory `json:"orbCategories"`
}

// SortBy allows us to sort a collection of orbs by builds, projects, or orgs from the last 30 days of data.
func (orbs *OrbsForListing) SortBy(sortBy string) {
	switch sortBy {
	case "builds":
		sort.Slice(orbs.Orbs, func(i, j int) bool {
			return orbs.Orbs[i].Statistics.Last30DaysBuildCount > orbs.Orbs[j].Statistics.Last30DaysBuildCount
		})
	case "projects":
		sort.Slice(orbs.Orbs, func(i, j int) bool {
			return orbs.Orbs[i].Statistics.Last30DaysProjectCount > orbs.Orbs[j].Statistics.Last30DaysProjectCount
		})
	case "orgs":
		sort.Slice(orbs.Orbs, func(i, j int) bool {
			return orbs.Orbs[i].Statistics.Last30DaysOrganizationCount > orbs.Orbs[j].Statistics.Last30DaysOrganizationCount
		})
	}
}

// OrbBase represents the minimum fields we wish to serialize for orbs.
// This type can be embedded for extending orbs with more data. e.g. OrbWithData
type OrbBase struct {
	Name           string        `json:"name"`
	HighestVersion string        `json:"version"`
	Statistics     OrbStatistics `json:"statistics"`
	Versions       []struct {
		Version string `json:"version"`
		Source  string `json:"source"`
	} `json:"versions"`
}

// OrbStatistics represents the data we retrieve for orb usage in the last thirty days.
type OrbStatistics struct {
	Last30DaysBuildCount        int `json:"last30DaysBuildCount"`
	Last30DaysProjectCount      int `json:"last30DaysProjectCount"`
	Last30DaysOrganizationCount int `json:"last30DaysOrganizationCount"`
}

// OrbWithData extends the OrbBase type with additional data used for printing.
type OrbWithData struct {
	OrbBase

	Commands  map[string]OrbElement
	Jobs      map[string]OrbElement
	Executors map[string]OrbElement
}

// MarshalJSON allows us to leave out excess fields we don't want to serialize.
// As is the case with commands/jobs/executors and now statistics.
func (orb OrbWithData) MarshalJSON() ([]byte, error) {
	orbForJSON := OrbBase{
		orb.Name,
		orb.HighestVersion,
		orb.Statistics,
		orb.Versions,
	}

	return json.Marshal(orbForJSON)
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

// Namespace represents the contents of a single namespace.
type Namespace struct {
	Name string
}

// Orb is a struct for containing the yaml-unmarshaled contents of an orb
type Orb struct {
	ID        string
	Name      string
	Namespace Namespace
	CreatedAt string

	Source         string
	HighestVersion string `json:"version"`

	Statistics struct {
		Last30DaysBuildCount        int
		Last30DaysProjectCount      int
		Last30DaysOrganizationCount int
	}

	Commands  map[string]OrbElement
	Jobs      map[string]OrbElement
	Executors map[string]OrbElement
	Versions  []OrbVersion

	Categories []OrbCategory
}

// Shortname returns the orb's name without its associated namespace.
func (o *Orb) Shortname() string {
	_, orbName, err := references.SplitIntoOrbAndNamespace(o.Name)
	if err != nil {
		panic(err)
	}

	return orbName
}

// OrbVersion wraps the GQL result used by OrbSource and OrbInfo
type OrbVersion struct {
	ID        string
	Version   string
	Orb       Orb
	Source    string
	CreatedAt string
}

type OrbCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FollowedProject struct {
	Followed bool   `json:"followed"`
	Message  string `json:"message"`
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
func WhoamiQuery(cl *graphql.Client) (*WhoamiResponse, error) {
	response := WhoamiResponse{}
	query := `query { me { name } }`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	err := cl.Run(request, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// ConfigQueryLegacy calls the GQL API to validate and process config with the legacy orgSlug
func ConfigQueryLegacy(cl *graphql.Client, configPath string, orgSlug string, params pipeline.Parameters, values pipeline.Values) (*ConfigResponse, error) {
	var response BuildConfigResponse
	var query string
	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}
	// GraphQL isn't forwards-compatible, so we are unusually selective here about
	// passing only non-empty fields on to the API, to minimize user impact if the
	// backend is out of date.
	var fieldAddendums string
	if orgSlug != "" {
		fieldAddendums += ", orgSlug: $orgSlug"
	}
	if len(params) > 0 {
		fieldAddendums += ", pipelineParametersJson: $pipelineParametersJson"
	}
	query = fmt.Sprintf(
		`query ValidateConfig ($config: String!, $pipelineParametersJson: String, $pipelineValues: [StringKeyVal!], $orgSlug: String) {
			buildConfig(configYaml: $config, pipelineValues: $pipelineValues%s) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`,
		fieldAddendums)

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("config", config)

	if values != nil {
		request.Var("pipelineValues", pipeline.PrepareForGraphQL(values))
	}
	if params != nil {
		pipelineParameters, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("unable to serialize pipeline values: %s", err.Error())
		}
		request.Var("pipelineParametersJson", string(pipelineParameters))
	}

	if orgSlug != "" {
		request.Var("orgSlug", orgSlug)
	}

	err = cl.Run(request, &response)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}
	if len(response.BuildConfig.ConfigResponse.Errors) > 0 {
		return nil, &response.BuildConfig.ConfigResponse.Errors
	}

	return &response.BuildConfig.ConfigResponse, nil
}

// ConfigQuery calls the GQL API to validate and process config with the org id
func ConfigQuery(cl *graphql.Client, configPath string, orgId string, params pipeline.Parameters, values pipeline.Values) (*ConfigResponse, error) {
	var response BuildConfigResponse
	var query string
	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}
	// GraphQL isn't forwards-compatible, so we are unusually selective here about
	// passing only non-empty fields on to the API, to minimize user impact if the
	// backend is out of date.
	var fieldAddendums string
	if orgId != "" {
		fieldAddendums += ", orgId: $orgId"
	}
	if len(params) > 0 {
		fieldAddendums += ", pipelineParametersJson: $pipelineParametersJson"
	}
	query = fmt.Sprintf(
		`query ValidateConfig ($config: String!, $pipelineParametersJson: String, $pipelineValues: [StringKeyVal!], $orgId: UUID!) {
			buildConfig(configYaml: $config, pipelineValues: $pipelineValues%s) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`,
		fieldAddendums)

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("config", config)

	if values != nil {
		request.Var("pipelineValues", pipeline.PrepareForGraphQL(values))
	}
	if params != nil {
		pipelineParameters, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("unable to serialize pipeline values: %s", err.Error())
		}
		request.Var("pipelineParametersJson", string(pipelineParameters))
	}

	if orgId != "" {
		request.Var("orgId", orgId)
	}

	err = cl.Run(request, &response)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}
	if len(response.BuildConfig.ConfigResponse.Errors) > 0 {
		return nil, &response.BuildConfig.ConfigResponse.Errors
	}

	return &response.BuildConfig.ConfigResponse, nil
}

// OrbQuery validated and processes an orb.
func OrbQuery(cl *graphql.Client, configPath string) (*ConfigResponse, error) {
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

	request := graphql.NewRequest(query)
	request.Var("config", config)
	request.SetToken(cl.Token)

	err = cl.Run(request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}

	if len(response.OrbConfig.ConfigResponse.Errors) > 0 {
		return nil, response.OrbConfig.ConfigResponse.Errors
	}

	return &response.OrbConfig.ConfigResponse, nil
}

// OrbImportVersion publishes a new version of an orb using the provided source and id.
func OrbImportVersion(cl *graphql.Client, orbSrc string, orbID string, orbVersion string) (*Orb, error) {
	var response OrbImportVersionResponse

	query := `
		mutation($config: String!, $orbId: UUID!, $version: String!) {
			importOrbVersion(
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("config", orbSrc)
	request.Var("orbId", orbID)
	request.Var("version", orbVersion)

	err := cl.Run(request, &response)
	if err != nil {
		return nil, errors.Wrap(err, "unable to import orb version")
	}

	if len(response.ImportOrbVersion.Errors) > 0 {
		return nil, response.ImportOrbVersion.Errors
	}

	return &response.ImportOrbVersion.Orb, nil
}

// OrbPublishByName publishes a new version of an orb using the provided orb's name and namespace, returning any
// error encountered.
func OrbPublishByName(cl *graphql.Client, configPath, orbName, namespaceName, orbVersion string) (*Orb, error) {
	var response OrbPublishResponse

	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	query := `
		mutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {
			publishOrb(
				orbName: $orbName,
				namespaceName: $namespaceName,
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("config", config)
	request.Var("orbName", orbName)
	request.Var("namespaceName", namespaceName)
	request.Var("version", orbVersion)

	err = cl.Run(request, &response)

	if err != nil {
		return nil, errors.Wrap(err, "Unable to publish orb")
	}

	if len(response.PublishOrb.Errors) > 0 {
		return nil, response.PublishOrb.Errors
	}

	return &response.PublishOrb.Orb, nil
}

// OrbExists checks whether an orb exists within the provided namespace and whether it's private.
func OrbExists(cl *graphql.Client, namespace string, orb string) (bool, bool, error) {
	name := namespace + "/" + orb

	var response OrbIDResponse

	query := `
	query ($name: String!, $namespace: String) {
		orb(name: $name) {
		  id
		  isPrivate
		}
		registryNamespace(name: $namespace) {
			id
		  }
	  }
	  `

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("name", name)
	request.Var("namespace", namespace)

	err := cl.Run(request, &response)
	if err != nil {
		return false, false, err
	}

	return response.Orb.ID != "", response.Orb.IsPrivate, nil
}

// OrbID fetches an orb returning the ID
func OrbID(cl *graphql.Client, namespace string, orb string) (*OrbIDResponse, error) {
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)
	request.Var("namespace", namespace)

	err := cl.Run(request, &response)

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

// CreateImportedNamespace creates an imported namespace with the provided name. An imported namespace
// does not require organization-level details.
func CreateImportedNamespace(cl *graphql.Client, name string) (*ImportNamespaceResponse, error) {
	var response ImportNamespaceResponse

	query := `
			mutation($name: String!) {
				importNamespace(
					name: $name,
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)

	err := cl.Run(request, &response)
	if err != nil {
		return nil, err
	}

	if len(response.ImportNamespace.Errors) > 0 {
		return nil, response.ImportNamespace.Errors
	}

	return &response, nil
}

func CreateNamespaceWithOwnerID(cl *graphql.Client, name string, ownerID string) (*CreateNamespaceResponse, error) {
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)
	request.Var("organizationId", ownerID)

	err := cl.Run(request, &response)

	if len(response.CreateNamespace.Errors) > 0 {
		return nil, response.CreateNamespace.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func getOrganization(cl *graphql.Client, organizationName string, organizationVcs string) (*GetOrganizationResponse, error) {
	var response GetOrganizationResponse

	query := `query($organizationName: String!, $organizationVcs: VCSType!) {
				organization(
					name: $organizationName
					vcsType: $organizationVcs
				) {
					id
				}
			}`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("organizationName", organizationName)
	request.Var("organizationVcs", strings.ToUpper(organizationVcs))

	err := cl.Run(request, &response)

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

func DeleteNamespaceAlias(cl *graphql.Client, name string) error {
	var response struct {
		DeleteNamespaceAlias struct {
			Deleted bool
			Errors  GQLErrorsCollection
		}
	}
	query := `
mutation($name: String!) {
  deleteNamespaceAlias(name: $name) {
    deleted
    errors {
      type
      message
    }
  }
}
`
	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)
	err := cl.Run(request, &response)
	if err != nil {
		return err
	}

	if len(response.DeleteNamespaceAlias.Errors) > 0 {
		return response.DeleteNamespaceAlias.Errors
	}

	if !response.DeleteNamespaceAlias.Deleted {
		return errors.New("Namespace alias deletion failed for unknown reasons.")
	}

	return nil
}

func DeleteNamespace(cl *graphql.Client, id string) error {
	var response struct {
		DeleteNamespace struct {
			Deleted bool
			Errors  GQLErrorsCollection
		} `json:"deleteNamespaceAndRelatedOrbs"`
	}
	query := `
mutation($id: UUID!) {
  deleteNamespaceAndRelatedOrbs(namespaceId: $id) {
    deleted
    errors {
      type
      message
    }
  }
}
`
	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("id", id)

	err := cl.Run(request, &response)
	if err != nil {
		return err
	}

	if len(response.DeleteNamespace.Errors) > 0 {
		return response.DeleteNamespace.Errors
	}

	if !response.DeleteNamespace.Deleted {
		return errors.New("Namespace deletion failed for unknown reasons.")
	}

	return nil
}

// CreateNamespace creates (reserves) a namespace for an organization
func CreateNamespace(cl *graphql.Client, name string, organizationName string, organizationVcs string) (*CreateNamespaceResponse, error) {
	getOrgResponse, getOrgError := getOrganization(cl, organizationName, organizationVcs)

	if getOrgError != nil {
		return nil, errors.Wrap(organizationNotFound(organizationName, organizationVcs), getOrgError.Error())
	}

	createNSResponse, createNSError := CreateNamespaceWithOwnerID(cl, name, getOrgResponse.Organization.ID)

	if createNSError != nil {
		return nil, createNSError
	}

	return createNSResponse, nil
}

func GetNamespace(cl *graphql.Client, name string) (*GetNamespaceResponse, error) {
	var response GetNamespaceResponse

	query := `
				query($name: String!) {
					registryNamespace(
						name: $name
					){
						id
					}
			 }`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)

	if err := cl.Run(request, &response); err != nil {
		return nil, errors.Wrapf(err, "failed to load namespace '%s'", err)
	}

	if response.RegistryNamespace.ID == "" {
		return nil, namespaceNotFound(name)
	}

	return &response, nil
}

// NamespaceExists returns a boolean indicating if the provided namespace exists.
func NamespaceExists(cl *graphql.Client, namespace string) (bool, error) {
	var response GetNamespaceResponse

	query := `
				query($name: String!) {
					registryNamespace(
						name: $name
					){
						id
					}
			 }`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("name", namespace)

	if err := cl.Run(request, &response); err != nil {
		return false, errors.Wrapf(err, "failed to load namespace '%s'", err)
	}

	if response.RegistryNamespace.ID != "" {
		return true, nil
	}

	return false, nil
}

func renameNamespaceWithNsID(cl *graphql.Client, id, newName string) (*RenameNamespaceResponse, error) {
	var response RenameNamespaceResponse

	query := `
		mutation($namespaceId: UUID!, $newName: String!){
			renameNamespace(
				namespaceId: $namespaceId,
				newName: $newName
			){
				namespace {
					id
				}
				errors {
					message
					type
				}
			}
		}`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("namespaceId", id)
	request.Var("newName", newName)

	err := cl.Run(request, &response)

	if len(response.RenameNamespace.Errors) > 0 {
		return nil, response.RenameNamespace.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func RenameNamespace(cl *graphql.Client, oldName, newName string) (*RenameNamespaceResponse, error) {
	getNamespaceResponse, err := GetNamespace(cl, oldName)
	if err != nil {
		return nil, err
	}
	return renameNamespaceWithNsID(cl, getNamespaceResponse.RegistryNamespace.ID, newName)
}

func createOrbWithNsID(cl *graphql.Client, name string, namespaceID string, isPrivate bool) (*CreateOrbResponse, error) {
	var response CreateOrbResponse

	query := `mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){
				createOrb(
					name: $name,
					registryNamespaceId: $registryNamespaceId,
					isPrivate: $isPrivate
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)
	request.Var("registryNamespaceId", namespaceID)
	request.Var("isPrivate", isPrivate)

	err := cl.Run(request, &response)

	if len(response.CreateOrb.Errors) > 0 {
		return nil, response.CreateOrb.Errors
	}

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateOrb creates (reserves) an orb within a namespace
func CreateOrb(cl *graphql.Client, namespace string, name string, isPrivate bool) (*CreateOrbResponse, error) {
	response, err := GetNamespace(cl, namespace)
	if err != nil {
		return nil, err
	}

	return createOrbWithNsID(cl, name, response.RegistryNamespace.ID, isPrivate)
}

// CreateImportedOrb creates (reserves) an imported orb within the provided namespace.
func CreateImportedOrb(cl *graphql.Client, namespace string, name string) (*ImportOrbResponse, error) {
	res, err := GetNamespace(cl, namespace)
	if err != nil {
		return nil, err
	}

	var response ImportOrbResponse

	query := `mutation($name: String!, $registryNamespaceId: UUID!){
				importOrb(
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)
	request.Var("registryNamespaceId", res.RegistryNamespace.ID)

	err = cl.Run(request, &response)
	if err != nil {
		return nil, err
	}

	if len(response.ImportOrb.Errors) > 0 {
		return nil, response.ImportOrb.Errors
	}

	return &response, nil
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
func OrbIncrementVersion(cl *graphql.Client, configPath string, namespace string, orb string, segment string) (*Orb, error) {
	v, err := OrbLatestVersion(cl, namespace, orb)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	response, err := OrbPublishByName(cl, configPath, orb, namespace, v2)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// OrbLatestVersion finds the latest published version of an orb and returns it.
// If it doesn't find a version, it will return 0.0.0 for the orb's version
func OrbLatestVersion(cl *graphql.Client, namespace string, orb string) (string, error) {
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("name", name)

	err := cl.Run(request, &response)
	if err != nil {
		return "", err
	}

	if len(response.Orb.Versions) != 1 {
		return "0.0.0", nil
	}

	return response.Orb.Versions[0].Version, nil
}

// OrbPromoteByName utilizes the given orb's name, namespace, development version, and segment to increment a semantic release.
func OrbPromoteByName(cl *graphql.Client, namespaceName, orbName, label, segment string) (*Orb, error) {
	v, err := OrbLatestVersion(cl, namespaceName, orbName)
	if err != nil {
		return nil, err
	}

	v2, err := incrementVersion(v, segment)
	if err != nil {
		return nil, err
	}

	var response OrbPromoteResponse

	query := `
		mutation($orbName: String, $namespaceName: String, $devVersion: String!, $semanticVersion: String!) {
			promoteOrb(
				orbName: $orbName,
				namespaceName: $namespaceName,
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("orbName", orbName)
	request.Var("namespaceName", namespaceName)
	request.Var("devVersion", label)
	request.Var("semanticVersion", v2)

	err = cl.Run(request, &response)

	if len(response.PromoteOrb.Errors) > 0 {
		return nil, response.PromoteOrb.Errors
	}

	if err != nil {
		return nil, errors.Wrap(err, "Unable to promote orb")
	}

	return &response.PromoteOrb.Orb, nil
}

// OrbSetOrbListStatus sets whether an orb can be listed in the registry or not.
func OrbSetOrbListStatus(cl *graphql.Client, namespace string, orb string, list bool) (*bool, error) {
	id, err := OrbID(cl, namespace, orb)
	if err != nil {
		return nil, err
	}

	var response OrbSetOrbListStatusResponse

	query := `
		mutation($orbId: UUID!, $list: Boolean!) {
			setOrbListStatus(
				orbId: $orbId,
				list: $list
			) {
				listed
				errors { 
					message
					type 
				}
			}
		}
	`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("orbId", id.Orb.ID)
	request.Var("list", list)

	err = cl.Run(request, &response)

	if len(response.SetOrbListStatus.Errors) > 0 {
		return nil, response.SetOrbListStatus.Errors
	}

	if err != nil {
		return nil, errors.Wrap(err, "Unable to set orb list status")
	}

	return &response.SetOrbListStatus.Listed, nil
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
func OrbSource(cl *graphql.Client, orbRef string) (string, error) {
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("orbVersionRef", ref)

	err := cl.Run(request, &response)
	if err != nil {
		return "", err
	}

	if response.OrbVersion.ID == "" {
		return "", fmt.Errorf("no Orb '%s' was found; please check that the Orb reference is correct", orbRef)
	}

	return response.OrbVersion.Source, nil
}

// ErrOrbVersionNotExists is a custom error type that communicates that
// an orb version was not found.
type ErrOrbVersionNotExists struct {
	OrbRef string
}

// Error implements the standard error interface.
func (e *ErrOrbVersionNotExists) Error() string {
	return fmt.Sprintf("no Orb '%s' was found; please check that the Orb reference is correct", e.OrbRef)
}

// OrbInfo gets the meta-data of an orb
func OrbInfo(cl *graphql.Client, orbRef string) (*OrbVersion, error) {
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
									namespace {
									  name
									}
                                    categories {
                                      id
                                      name
                                    }
	                            statistics {
		                        last30DaysBuildCount,
		                        last30DaysProjectCount,
		                        last30DaysOrganizationCount
	                            }
                                    versions(count: 200) {
                                        createdAt
                                        version
                                    }
                                }
                                source
                                createdAt
			    }
		      }`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("orbVersionRef", ref)

	err := cl.Run(request, &response)
	if err != nil {
		return nil, err
	}

	if response.OrbVersion.ID == "" {
		return nil, &ErrOrbVersionNotExists{
			OrbRef: ref,
		}
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
func ListOrbs(cl *graphql.Client, uncertified bool) (*OrbsForListing, error) {
	l := log.New(os.Stderr, "", 0)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
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
		request := graphql.NewRequest(query)
		request.SetToken(cl.Token)
		request.Var("after", currentCursor)
		request.Var("certifiedOnly", !uncertified)

		err := cl.Run(request, &result)
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

// ListNamespaceOrbVersions queries the API to retrieve the orbs belonging to the given namespace.
// By default, this call fetches the latest version of each orb.
func ListNamespaceOrbVersions(cl *graphql.Client, namespace string) ([]OrbVersion, error) {
	query := `
query namespaceOrbs ($namespace: String, $after: String!) {
		registryNamespace(name: $namespace) {
			name
			id
			orbs(first: 20, after: $after) {
				edges {
					cursor
					node {
						versions(count: 1) {
							source
							id
							version
							createdAt
						}
						name
						id
						createdAt
					}
				}
				pageInfo {
					hasNextPage
				}
			}
		}
	}
`
	var orbVersions []OrbVersion
	var result NamespaceOrbVersionResponse
	var currentCursor string

	for {
		request := graphql.NewRequest(query)
		request.SetToken(cl.Token)
		request.Var("after", currentCursor)
		request.Var("namespace", namespace)

		err := cl.Run(request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

		if result.RegistryNamespace.ID == "" {
			return nil, errors.New("No namespace found")
		}

		for _, edge := range result.RegistryNamespace.Orbs.Edges {
			currentCursor = edge.Cursor

			orb := Orb{
				Name: edge.Node.Name,
				Namespace: Namespace{
					Name: result.RegistryNamespace.Name,
				},
			}

			for _, v := range edge.Node.Versions {
				v.Orb = orb
				orbVersions = append(orbVersions, v)
			}
		}

		if !result.RegistryNamespace.Orbs.PageInfo.HasNextPage {
			break
		}
	}

	return orbVersions, nil
}

// ListNamespaceOrbs queries the API to find all orbs belonging to the given
// namespace.
// Returns a collection of Orb objects containing their relevant data.
func ListNamespaceOrbs(cl *graphql.Client, namespace string, isPrivate, showDetails bool) (*OrbsForListing, error) {
	l := log.New(os.Stderr, "", 0)

	query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions `

	if showDetails {
		query += `(count: 1){ source,`
	} else {
		query += `{`
	}

	query += ` version
					}
					name
	                                statistics {
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

	view := "PUBLIC_ONLY"
	if isPrivate {
		view = "PRIVATE_ONLY"
	}

	for {
		request := graphql.NewRequest(query)
		request.SetToken(cl.Token)
		request.Var("after", currentCursor)
		request.Var("namespace", namespace)
		request.Var("view", view)

		orbs.Namespace = namespace

		err := cl.Run(request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

		if result.RegistryNamespace.ID == "" {
			return nil, errors.New("No namespace found")
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
func IntrospectionQuery(cl *graphql.Client) (*IntrospectionResponse, error) {
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

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	err := cl.Run(request, &response)

	return &response, err
}

// OrbCategoryID fetches an orb returning the ID
func OrbCategoryID(cl *graphql.Client, name string) (*OrbCategoryIDResponse, error) {
	var response OrbCategoryIDResponse

	query := `
	query ($name: String!) {
		orbCategoryByName(name: $name) {
		  id
		}
	}`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("name", name)

	err := cl.Run(request, &response)

	// If there is an error, or the request was successful, return now.
	if err != nil || response.OrbCategoryByName.ID != "" {
		return &response, err
	}

	return nil, fmt.Errorf("the '%s' category does not exist. Did you misspell the category name? To see the list of category names, please run 'circleci orb list-categories'.", name)
}

// AddOrRemoveOrbCategorization adds or removes an orb categorization
func AddOrRemoveOrbCategorization(cl *graphql.Client, namespace string, orb string, categoryName string, updateType UpdateOrbCategorizationRequestType) error {
	orbId, err := OrbID(cl, namespace, orb)
	if err != nil {
		return err
	}

	categoryId, err := OrbCategoryID(cl, categoryName)
	if err != nil {
		return err
	}

	var response AddOrRemoveOrbCategorizationResponse

	var mutationName string
	if updateType == Add {
		mutationName = "addCategorizationToOrb"
	} else if updateType == Remove {
		mutationName = "removeCategorizationFromOrb"
	}

	if mutationName == "" {
		return fmt.Errorf("Internal error - invalid update type %d", updateType)
	}

	query := fmt.Sprintf(`
		mutation($orbId: UUID!, $categoryId: UUID!) {
			%s(
				orbId: $orbId,
				categoryId: $categoryId
			) {
				orbId
				categoryId
				errors {
					message
					type
				}
			}
		}
	`, mutationName)

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("orbId", orbId.Orb.ID)
	request.Var("categoryId", categoryId.OrbCategoryByName.ID)

	err = cl.Run(request, &response)

	responseData := response[mutationName]

	if len(responseData.Errors) > 0 {
		return &responseData.Errors
	}

	if err != nil {
		return errors.Wrap(err, "Unable to add/remove orb categorization")
	}

	return nil
}

// ListOrbCategories queries the API to find all categories.
// Returns a collection of OrbCategory objects containing their relevant data.
func ListOrbCategories(cl *graphql.Client) (*OrbCategoriesForListing, error) {

	query := `
	query ListOrbCategories($after: String!) {
		orbCategories(first: 20, after: $after) {
			totalCount
			edges {
				cursor
				node {
					id
					name
				}
			}
			pageInfo {
				hasNextPage
			}
		}
	}
`

	var orbCategories OrbCategoriesForListing

	var result OrbCategoryListResponse
	currentCursor := ""

	for {
		request := graphql.NewRequest(query)
		request.SetToken(cl.Token)
		request.Var("after", currentCursor)

		err := cl.Run(request, &result)
		if err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

		for i := range result.OrbCategories.Edges {
			edge := result.OrbCategories.Edges[i]
			currentCursor = edge.Cursor
			orbCategories.OrbCategories = append(orbCategories.OrbCategories, edge.Node)
		}

		if !result.OrbCategories.PageInfo.HasNextPage {
			break
		}

	}
	return &orbCategories, nil
}

// FollowProject initiates an API request to follow a specific project on
// CircleCI. Project slugs are case-sensitive.

var errorMessage = `Unable to follow project`

func FollowProject(config settings.Config, vcs string, owner string, projectName string) (FollowedProject, error) {

	requestPath := fmt.Sprintf("%s/api/v1.1/project/%s/%s/%s/follow", config.Host, vcs, owner, projectName)
	r, err := http.NewRequest(http.MethodPost, requestPath, nil)
	if err != nil {
		return FollowedProject{}, errors.Wrap(err, errorMessage)
	}
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.Header.Set("Accept", "application/json; charset=utf-8")
	r.Header.Set("Circle-Token", config.Token)

	response, err := config.HTTPClient.Do(r)
	if err != nil {
		return FollowedProject{}, err
	}
	if response.StatusCode >= 400 {
		return FollowedProject{}, errors.New("Could not follow project")
	}

	var fr FollowedProject
	err = json.NewDecoder(response.Body).Decode(&fr)
	if err != nil {
		return FollowedProject{}, errors.Wrap(err, errorMessage)
	}

	return fr, nil
}
