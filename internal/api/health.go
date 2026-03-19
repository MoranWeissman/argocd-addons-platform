package api

import (
	"net/http"
	"os"
	"strings"
)

// appVersion is read once at startup from the VERSION file.
var appVersion = readVersionFile()

func readVersionFile() string {
	// Try common locations
	for _, path := range []string{"version.txt", "/app/version.txt", "VERSION", "/app/VERSION"} {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return "dev"
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": appVersion,
	})
}
