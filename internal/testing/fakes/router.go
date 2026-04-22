package fakes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// newRouter creates a chi router with recovery middleware.
// All fake servers share this setup.
func newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	// Force chi to route on the decoded r.URL.Path rather than r.URL.RawPath,
	// so routes with literal slashes in path segments (e.g. vcs/org/repo) match correctly.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.RawPath = ""
			next.ServeHTTP(w, r)
		})
	})
	return r
}
