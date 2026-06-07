// Package rest contains the HTTP adapter that implements port.JiraClient
// against the Jira REST API v3. It depends only on domain/evidence and stdlib —
// zero transitive dependencies (REQ-AUTH, Design §ZERO transitive deps).
package rest

import (
	"encoding/base64"
	"net/http"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

// basicAuthHeader returns the value for the Authorization header using HTTP
// Basic authentication: "Basic base64(email:token)" (REQ-AUTH).
func basicAuthHeader(creds service.Credentials) string {
	raw := creds.Email + ":" + creds.APIToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// setBasicAuth attaches the Authorization header to req using the supplied
// credentials. This is a convenience wrapper so every request builder can call
// it in one line.
func setBasicAuth(req *http.Request, creds service.Credentials) {
	req.Header.Set("Authorization", basicAuthHeader(creds))
}
