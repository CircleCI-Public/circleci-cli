// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package fakes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

const (
	notificationNotFoundTitle        = "Resource does not exist or unauthorized"
	slackChannelNotFoundTitle        = "Channel not found. Verify the channel ID and run /invite @circleci in the channel to add the CircleCI app."
	slackConnectionInvalidTitle      = "Slack connection is no longer valid. Reconnect Slack and try again."
	slackCannotPostTitle             = "CircleCI does not have permission to post in this channel."
	notificationBadRequestTitle      = "bad request"
	notificationChannelTypeSlack     = "slack"
	notificationScopeProject         = "project"
	notificationIntegrationTypeSlack = "slack"
)

type SlackWorkspace struct {
	OrgID             string
	IntegrationID     string
	Name              string
	ConnectionInvalid bool
	Channels          []SlackChannel
}

type SlackChannel struct {
	ID         string
	Name       string
	Private    bool
	CannotPost bool
}

type ChannelConfigV3 struct {
	ID            string
	ProjectID     string
	OrgID         string
	ChannelType   string
	Target        string
	Enabled       bool
	DisableReason string
}

type NotificationPreferenceV3 struct {
	ID      string
	Name    string
	Group   string
	Channel string
	Enabled bool
}

func (f *CircleCI) AddSlackWorkspace(ws SlackWorkspace) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.slackWorkspaces[ws.OrgID] = ws
}

func (f *CircleCI) AddChannelConfig(cfg ChannelConfigV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putChannelConfigLocked(cfg)
}

func (f *CircleCI) ChannelConfig(projectID, channelType string) (ChannelConfigV3, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	cfg, ok := f.channelConfigs[channelConfigKey(projectID, channelType)]
	return cfg, ok
}

func (f *CircleCI) SetNotificationPreferences(projectID string, prefs ...NotificationPreferenceV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notificationPrefs[projectID] = prefs
}

func (f *CircleCI) DenyNotificationAccess(projectID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notificationDenied[projectID] = true
}

func (f *CircleCI) SlackWelcomeMessages() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]string(nil), f.slackWelcomePosts...)
}

func channelConfigKey(projectID, channelType string) string {
	return projectID + "/" + channelType
}

func (f *CircleCI) putChannelConfigLocked(cfg ChannelConfigV3) {
	f.channelConfigs[channelConfigKey(cfg.ProjectID, cfg.ChannelType)] = cfg
}

func (f *CircleCI) channelConfigByIDLocked(id string) (ChannelConfigV3, bool) {
	for _, cfg := range f.channelConfigs {
		if cfg.ID == id {
			return cfg, true
		}
	}
	return ChannelConfigV3{}, false
}

func (f *CircleCI) projectInOrgLocked(projectID, orgID string) bool {
	for _, p := range f.projectsBySlug {
		if p.ID == projectID && p.OrgID == orgID {
			return true
		}
	}
	return false
}

func (f *CircleCI) postSlackWelcomeLocked(orgID, target string) (string, int, string) {
	ws, ok := f.slackWorkspaces[orgID]
	if !ok {
		return "", http.StatusNotFound, notificationNotFoundTitle
	}
	if ws.ConnectionInvalid {
		return "", http.StatusUnauthorized, slackConnectionInvalidTitle
	}
	name := strings.TrimPrefix(target, "#")
	for _, ch := range ws.Channels {
		if ch.ID != target && (ch.Private || ch.Name != name) {
			continue
		}
		if ch.CannotPost {
			return "", http.StatusForbidden, slackCannotPostTitle
		}
		f.slackWelcomePosts = append(f.slackWelcomePosts, ch.ID)
		return ch.ID, 0, ""
	}
	return "", http.StatusNotFound, slackChannelNotFoundTitle
}

func (f *CircleCI) slackChannelNameLocked(orgID, channelID string) string {
	for _, ch := range f.slackWorkspaces[orgID].Channels {
		if ch.ID == channelID && !ch.Private {
			return ch.Name
		}
	}
	return ""
}

func (f *CircleCI) channelConfigEntityLocked(cfg ChannelConfigV3) map[string]any {
	attrs := map[string]any{
		"channel_type": cfg.ChannelType,
		"is_enabled":   cfg.Enabled,
	}
	if cfg.Target != "" {
		attrs["target"] = cfg.Target
		if cfg.ChannelType == notificationChannelTypeSlack {
			attrs["channel_name"] = f.slackChannelNameLocked(cfg.OrgID, cfg.Target)
		}
	}
	if cfg.DisableReason != "" {
		attrs["disable_reason"] = cfg.DisableReason
	}
	return map[string]any{
		"id":         cfg.ID,
		"attributes": attrs,
		"references": map[string]any{
			"project": map[string]any{"id": cfg.ProjectID},
			"org":     map[string]any{"id": cfg.OrgID},
		},
	}
}

func renderNotificationError(w http.ResponseWriter, r *http.Request, status int, title string) {
	render.Status(r, status)
	render.JSON(w, r, map[string]any{"error": map[string]any{"title": title}})
}

func (f *CircleCI) handleListChannelConfigsV3(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("filter[scope]") != notificationScopeProject {
		renderNotificationError(w, r, http.StatusBadRequest, `filter[scope] must be "user" or "project"`)
		return
	}
	orgID, projectID := q.Get("filter[org_id]"), q.Get("filter[project_id]")

	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.notificationDenied[projectID] {
		renderNotificationError(w, r, http.StatusNotFound, notificationNotFoundTitle)
		return
	}
	data := []map[string]any{}
	for _, cfg := range f.channelConfigs {
		if cfg.OrgID == orgID && (projectID == "" || cfg.ProjectID == projectID) {
			data = append(data, f.channelConfigEntityLocked(cfg))
		}
	}
	render.JSON(w, r, map[string]any{
		"data": data,
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

type createChannelConfigBodyV3 struct {
	Data struct {
		Attributes struct {
			Scope       string `json:"scope"`
			ChannelType string `json:"channel_type"`
			Target      string `json:"target"`
			IsEnabled   bool   `json:"is_enabled"`
		} `json:"attributes"`
		References struct {
			Project struct {
				ID string `json:"id"`
			} `json:"project"`
			Org struct {
				ID string `json:"id"`
			} `json:"org"`
		} `json:"references"`
	} `json:"data"`
}

func (f *CircleCI) handleCreateChannelConfigV3(w http.ResponseWriter, r *http.Request) {
	var body createChannelConfigBodyV3
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		renderNotificationError(w, r, http.StatusBadRequest, notificationBadRequestTitle)
		return
	}
	attrs, refs := body.Data.Attributes, body.Data.References
	if attrs.Scope != notificationScopeProject || attrs.ChannelType != notificationChannelTypeSlack {
		renderNotificationError(w, r, http.StatusBadRequest, notificationBadRequestTitle)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.projectInOrgLocked(refs.Project.ID, refs.Org.ID) || f.notificationDenied[refs.Project.ID] {
		renderNotificationError(w, r, http.StatusNotFound, notificationNotFoundTitle)
		return
	}
	cfg, exists := f.channelConfigs[channelConfigKey(refs.Project.ID, attrs.ChannelType)]
	if !exists {
		f.channelConfigSeq++
		cfg = ChannelConfigV3{
			ID:          fmt.Sprintf("00000000-0000-4000-8000-%012d", f.channelConfigSeq),
			ProjectID:   refs.Project.ID,
			OrgID:       refs.Org.ID,
			ChannelType: attrs.ChannelType,
		}
	}
	if attrs.IsEnabled {
		if attrs.Target == "" {
			renderNotificationError(w, r, http.StatusBadRequest, notificationBadRequestTitle)
			return
		}
		channelID, status, title := f.postSlackWelcomeLocked(refs.Org.ID, attrs.Target)
		if status != 0 {
			renderNotificationError(w, r, status, title)
			return
		}
		cfg.Target = channelID
	}
	cfg.Enabled = attrs.IsEnabled
	cfg.DisableReason = ""
	f.putChannelConfigLocked(cfg)

	if !exists {
		render.Status(r, http.StatusCreated)
	}
	render.JSON(w, r, map[string]any{"data": f.channelConfigEntityLocked(cfg)})
}

type updateChannelConfigBodyV3 struct {
	Target    *string `json:"target"`
	IsEnabled *bool   `json:"is_enabled"`
}

func (f *CircleCI) handleUpdateChannelConfigV3(w http.ResponseWriter, r *http.Request) {
	var body updateChannelConfigBodyV3
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		renderNotificationError(w, r, http.StatusBadRequest, notificationBadRequestTitle)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	cfg, ok := f.channelConfigByIDLocked(chi.URLParam(r, "id"))
	if !ok {
		renderNotificationError(w, r, http.StatusNotFound, notificationNotFoundTitle)
		return
	}
	if body.Target != nil {
		cfg.Target = *body.Target
	}
	if body.IsEnabled != nil {
		cfg.Enabled = *body.IsEnabled
	}
	if cfg.Enabled && cfg.ChannelType == notificationChannelTypeSlack {
		channelID, status, title := f.postSlackWelcomeLocked(cfg.OrgID, cfg.Target)
		if status != 0 {
			renderNotificationError(w, r, status, title)
			return
		}
		cfg.Target = channelID
		cfg.DisableReason = ""
	}
	f.putChannelConfigLocked(cfg)
	render.JSON(w, r, map[string]any{"data": f.channelConfigEntityLocked(cfg)})
}

func (f *CircleCI) handleDeleteChannelConfigV3(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cfg, ok := f.channelConfigByIDLocked(chi.URLParam(r, "id"))
	if !ok {
		renderNotificationError(w, r, http.StatusNotFound, notificationNotFoundTitle)
		return
	}
	delete(f.channelConfigs, channelConfigKey(cfg.ProjectID, cfg.ChannelType))
	w.WriteHeader(http.StatusNoContent)
}

func (f *CircleCI) handleListNotificationPreferencesV3(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	orgID, projectID := q.Get("filter[org_id]"), q.Get("filter[project_id]")
	if q.Get("filter[scope]") != notificationScopeProject || orgID == "" || projectID == "" {
		renderNotificationError(w, r, http.StatusBadRequest, notificationBadRequestTitle)
		return
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.projectInOrgLocked(projectID, orgID) {
		renderNotificationError(w, r, http.StatusNotFound, notificationNotFoundTitle)
		return
	}
	data := []map[string]any{}
	for _, p := range f.notificationPrefs[projectID] {
		data = append(data, map[string]any{
			"id": p.ID,
			"attributes": map[string]any{
				"preference_name": p.Name,
				"group_name":      p.Group,
				"channel":         p.Channel,
				"scope":           notificationScopeProject,
				"is_enabled":      p.Enabled,
			},
		})
	}
	render.JSON(w, r, map[string]any{"data": data})
}

func (f *CircleCI) handleListNotificationIntegrationsV3(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("filter[type]")

	f.mu.RLock()
	defer f.mu.RUnlock()
	data := []map[string]any{}
	ws, ok := f.slackWorkspaces[q.Get("filter[org_id]")]
	if ok && (typ == "" || typ == notificationIntegrationTypeSlack) {
		status := "active"
		if ws.ConnectionInvalid {
			status = "inactive"
		}
		data = append(data, map[string]any{
			"id": ws.IntegrationID,
			"attributes": map[string]any{
				"type":           notificationIntegrationTypeSlack,
				"workspace_name": ws.Name,
				"status":         status,
			},
			"references": map[string]any{
				"org": map[string]any{"id": ws.OrgID},
			},
		})
	}
	render.JSON(w, r, map[string]any{"data": data})
}
