package version

import "os"

const defaultVersion = "dev"

// version is injected at link time via -ldflags (Docker build); overridden by APP_VERSION at runtime.
var version = defaultVersion

// Current returns the application version exposed by GET /version.
func Current() string {
	if v := os.Getenv("APP_VERSION"); v != "" {
		return v
	}
	return version
}

// Response is the JSON body for GET /version (Discovery-aligned).
type Response struct {
	Version string `json:"version"`
}

// Payload returns the version response document.
func Payload() Response {
	return Response{Version: Current()}
}
