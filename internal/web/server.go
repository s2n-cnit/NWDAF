// Package web provides HTTP server functionality for the NWDAF web dashboard
package web

import (
	"net/http"
)

// GetHandler returns an HTTP handler for serving static web UI files from the filesystem
func GetHandler(webDir string) http.Handler {
	return http.FileServer(http.Dir(webDir))
}
