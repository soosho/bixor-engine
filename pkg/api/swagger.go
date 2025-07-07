package api

import (
	"net/http"
	"io/ioutil"
	"path/filepath"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gopkg.in/yaml.v2"
)

// SwaggerInfo holds the swagger specification info
var SwaggerInfo = struct {
	Version     string
	Host        string
	BasePath    string
	Title       string
	Description string
}{
	Version:     "1.0.0",
	Host:        "localhost:8080",
	BasePath:    "/api/v1",
	Title:       "Bixor Exchange API",
	Description: "High-performance cryptocurrency exchange API",
}

// setupSwagger configures Swagger documentation routes
func setupSwagger(r *gin.Engine) {
	// Serve the static OpenAPI YAML file
	r.GET("/api/v1/openapi.yaml", func(c *gin.Context) {
		yamlPath := filepath.Join("docs", "swagger.yaml")
		yamlData, err := ioutil.ReadFile(yamlPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to read OpenAPI specification",
			})
			return
		}

		c.Header("Content-Type", "application/yaml")
		c.Data(http.StatusOK, "application/yaml", yamlData)
	})

	// Serve JSON version of the spec
	r.GET("/api/v1/openapi.json", func(c *gin.Context) {
		yamlPath := filepath.Join("docs", "swagger.yaml")
		yamlData, err := ioutil.ReadFile(yamlPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to read OpenAPI specification",
			})
			return
		}

		var spec interface{}
		err = yaml.Unmarshal(yamlData, &spec)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to parse OpenAPI specification",
			})
			return
		}

		c.JSON(http.StatusOK, spec)
	})

	// Serve Swagger UI
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/v1/openapi.json")))

	// Redirect /docs to /docs/
	r.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/")
	})

	// API documentation info endpoint
	r.GET("/api/v1/docs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"title":       SwaggerInfo.Title,
				"description": SwaggerInfo.Description,
				"version":     SwaggerInfo.Version,
				"docs_url":    "/docs/",
				"openapi_url": "/api/v1/openapi.json",
				"endpoints": gin.H{
					"swagger_ui":   "/docs/",
					"openapi_json": "/api/v1/openapi.json",
					"openapi_yaml": "/api/v1/openapi.yaml",
				},
			},
		})
	})
}

// GetSwaggerRoutes returns documentation-related routes info
func GetSwaggerRoutes() map[string]string {
	return map[string]string{
		"docs":         "/docs/ - Interactive API documentation (Swagger UI)",
		"openapi_json": "/api/v1/openapi.json - OpenAPI 3.0 JSON specification",
		"openapi_yaml": "/api/v1/openapi.yaml - OpenAPI 3.0 YAML specification",
		"api_info":     "/api/v1/docs - API documentation information",
	}
} 