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

package fakesegment

import (
	"context"
	"encoding/base64"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/segmentio/analytics-go/v3"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/logger"
)

type Service struct {
	http.Handler

	apiKey string

	batches []Batch
	mu      sync.RWMutex
}

type Batch struct {
	SentAt   time.Time         `json:"sentAt"`
	Messages []analytics.Track `json:"batch"`
}

func New(ctx context.Context, apiKey string) *Service {
	r := chi.NewRouter()
	r.Use(logger.Middleware(ctx))
	fs := &Service{
		Handler: r,
		apiKey:  basicAuth(apiKey, ""),
	}

	r.Post("/v1/batch", fs.handleBatch)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "not found"})
	})

	return fs
}

func (s *Service) handleBatch(w http.ResponseWriter, r *http.Request) {
	authZ := r.Header.Get("Authorization")
	if authZ != "Basic "+s.apiKey {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]any{"error": "unauthorized"})
		return
	}

	var sentBatch Batch
	err := render.DecodeJSON(r.Body, &sentBatch)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"error": "bad request"})
		return
	}

	s.mu.Lock()
	s.batches = append(s.batches, sentBatch)
	s.mu.Unlock()

	render.JSON(w, r, map[string]any{"success": true})
}

func (s *Service) Batches() []Batch {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Clone(s.batches)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

var (
	CompareTrack = cmpopts.IgnoreFields(analytics.Track{}, "MessageId")

	// CompareTime treats timestamps as equal when they are close, because a test
	// cannot know the instant the sender stamped an event or the instant this fake
	// received the batch.
	//
	// The window is deliberately loose. What these assertions are for is "a
	// plausible time was recorded" — that the field was populated at all, from the
	// clock rather than left zero — not sub-second precision. A tight window buys
	// no extra coverage and turns a slow CI box, or a `-race` run under load, into
	// a failure: flushing the batch and delivering it over HTTP is what separates
	// the two times, and that is not bounded by anything the test controls.
	CompareTime = cmpopts.EquateApproxTime(time.Minute)
)
