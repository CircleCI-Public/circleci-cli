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
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// newRouter creates a chi router with recovery middleware.
// All fake servers share this setup.
func newRouter() *chi.Mux {
	ctx := iostream.Testing(context.Background())

	r := chi.NewRouter()
	r.Use(structuredLogger(ctx))
	r.Use(middleware.Recoverer)
	r.Use(decodeRawPath)
	return r
}

// structuredLogger logs each request using iostream.DebugContext with the same
// attributes as the httpcl client logger.
func structuredLogger(ctx context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)
			next.ServeHTTP(ww, req)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			iostream.DebugContext(ctx, req.Method+" "+req.URL.Path,
				"http.request.method", req.Method,
				"http.response.status_code", status,
				"duration", time.Since(start),
				"url.full", req.URL.String(),
				"kind", "server",
			)
		})
	}
}

// decodeRawPath forces chi to route on the decoded r.URL.Path rather than
// r.URL.RawPath, so routes with literal slashes in path segments
// (e.g. vcs/org/repo) match correctly.
func decodeRawPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.RawPath = ""
		next.ServeHTTP(w, r)
	})
}
