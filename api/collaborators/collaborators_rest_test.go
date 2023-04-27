package collaborators_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/collaborators"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"gotest.tools/v3/assert"
)

func getCollaboratorsRestClient(server *httptest.Server) (collaborators.CollaboratorsClient, error) {
	client := &http.Client{}

	return collaborators.NewCollaboratorsRestClient(settings.Config{
		RestEndpoint: "api/v2",
		Host:         server.URL,
		HTTPClient:   client,
		Token:        "token",
	})
}

func Test_collaboratorsRestClient_GetOrgCollaborations(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    []collaborators.CollaborationResult
		wantErr bool
	}{
		{
			name: "Should handle a successful request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, "/api/v2/me/collaborations")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
					[
						{
							"vcs_type": "github",
							"slug": "gh/example",
							"name": "Example Org",
							"id": "some-uuid-123",
							"avatar_url": "http://placekitten.com/200/300"
						},
						{
							"vcs_type": "bitbucket",
							"slug": "bb/other",
							"name": "Other Org",
							"id": "some-uuid-789",
							"avatar_url": "http://placekitten.com/200/300"
						}
					]
				`))

				assert.NilError(t, err)
			},
			want: []collaborators.CollaborationResult{
				{
					VcsType:   "github",
					OrgSlug:   "gh/example",
					OrgName:   "Example Org",
					OrgId:     "some-uuid-123",
					AvatarUrl: "http://placekitten.com/200/300",
				},
				{
					VcsType:   "bitbucket",
					OrgSlug:   "bb/other",
					OrgName:   "Other Org",
					OrgId:     "some-uuid-789",
					AvatarUrl: "http://placekitten.com/200/300",
				},
			},
			wantErr: false,
		},
		{
			name: "Should handle an error request",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message": "error"}`))
				assert.NilError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			c, err := getCollaboratorsRestClient(server)
			assert.NilError(t, err)

			got, err := c.GetOrgCollaborations()
			if (err != nil) != tt.wantErr {
				t.Errorf("collaboratorsRestClient.GetOrgCollaborations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collaboratorsRestClient.GetOrgCollaborations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_collaboratorsRestClient_GetCollaborationBySlug(t *testing.T) {
	type args struct {
		slug string
	}
	tests := []struct {
		name    string
		handler http.HandlerFunc
		args    args
		want    *collaborators.CollaborationResult
		wantErr bool
	}{
		{
			name: "Should work with short-vcs notation (github)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, "/api/v2/me/collaborations")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
					[
						{
							"vcs_type": "github",
							"slug": "gh/example",
							"name": "Example Org",
							"id": "some-uuid-123",
							"avatar_url": "http://placekitten.com/200/300"
						},
						{
							"vcs_type": "bitbucket",
							"slug": "bb/other",
							"name": "Other Org",
							"id": "some-uuid-789",
							"avatar_url": "http://placekitten.com/200/300"
						}
					]
				`))

				assert.NilError(t, err)
			},
			args: args{
				slug: "gh/example",
			},
			want: &collaborators.CollaborationResult{
				VcsType:   "github",
				OrgSlug:   "gh/example",
				OrgName:   "Example Org",
				OrgId:     "some-uuid-123",
				AvatarUrl: "http://placekitten.com/200/300",
			},
			wantErr: false,
		},
		{
			name: "Should work with vcs-name notation (github)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, "/api/v2/me/collaborations")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
					[
						{
							"vcs_type": "github",
							"slug": "gh/example",
							"name": "Example Org",
							"id": "some-uuid-123",
							"avatar_url": "http://placekitten.com/200/300"
						},
						{
							"vcs_type": "bitbucket",
							"slug": "bb/other",
							"name": "Other Org",
							"id": "some-uuid-789",
							"avatar_url": "http://placekitten.com/200/300"
						}
					]
				`))

				assert.NilError(t, err)
			},
			args: args{
				slug: "github/example",
			},
			want: &collaborators.CollaborationResult{
				VcsType:   "github",
				OrgSlug:   "gh/example",
				OrgName:   "Example Org",
				OrgId:     "some-uuid-123",
				AvatarUrl: "http://placekitten.com/200/300",
			},
			wantErr: false,
		},
		{
			name: "Should work with vcs-short notation (bitbucket)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, "/api/v2/me/collaborations")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
					[
						{
							"vcs_type": "github",
							"slug": "gh/example",
							"name": "Example Org",
							"id": "some-uuid-123",
							"avatar_url": "http://placekitten.com/200/300"
						},
						{
							"vcs_type": "bitbucket",
							"slug": "bb/other",
							"name": "Other Org",
							"id": "some-uuid-789",
							"avatar_url": "http://placekitten.com/200/300"
						}
					]
				`))

				assert.NilError(t, err)
			},
			args: args{
				slug: "bb/other",
			},
			want: &collaborators.CollaborationResult{
				VcsType:   "bitbucket",
				OrgSlug:   "bb/other",
				OrgName:   "Other Org",
				OrgId:     "some-uuid-789",
				AvatarUrl: "http://placekitten.com/200/300",
			},
			wantErr: false,
		},
		{
			name: "Should work with vcs-name notation (bitbucket)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, "/api/v2/me/collaborations")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
					[
						{
							"vcs_type": "github",
							"slug": "gh/example",
							"name": "Example Org",
							"id": "some-uuid-123",
							"avatar_url": "http://placekitten.com/200/300"
						},
						{
							"vcs_type": "bitbucket",
							"slug": "bb/other",
							"name": "Other Org",
							"id": "some-uuid-789",
							"avatar_url": "http://placekitten.com/200/300"
						}
					]
				`))

				assert.NilError(t, err)
			},
			args: args{
				slug: "bitbucket/other",
			},
			want: &collaborators.CollaborationResult{
				VcsType:   "bitbucket",
				OrgSlug:   "bb/other",
				OrgName:   "Other Org",
				OrgId:     "some-uuid-789",
				AvatarUrl: "http://placekitten.com/200/300",
			},
			wantErr: false,
		},
		{
			name: "Should return nil if not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, "/api/v2/me/collaborations")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
					[
						{
							"vcs_type": "github",
							"slug": "gh/example",
							"name": "Example Org",
							"id": "some-uuid-123",
							"avatar_url": "http://placekitten.com/200/300"
						},
						{
							"vcs_type": "bitbucket",
							"slug": "bb/other",
							"name": "Other Org",
							"id": "some-uuid-789",
							"avatar_url": "http://placekitten.com/200/300"
						}
					]
				`))

				assert.NilError(t, err)
			},
			args: args{
				slug: "bad-slug",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Should error if request errors",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message": "error"}`))
				assert.NilError(t, err)
			},
			args: args{
				slug: "bad-slug",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			c, err := getCollaboratorsRestClient(server)
			assert.NilError(t, err)

			got, err := c.GetCollaborationBySlug(tt.args.slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("collaboratorsRestClient.GetCollaborationBySlug() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collaboratorsRestClient.GetCollaborationBySlug() = %v, want %v", got, tt.want)
			}
		})
	}
}
