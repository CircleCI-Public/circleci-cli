package fakes

import (
	"github.com/gin-gonic/gin"
)

// newRouter creates a gin router with recovery middleware.
// All fake servers share this setup.
func newRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}
