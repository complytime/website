// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// urlRewriter intercepts HTTP requests and redirects them to the test server,
// allowing the apiClient to use its hardcoded githubAPI constant while actually
// hitting the mock server.
type urlRewriter struct {
	targetHost string
	targetPort string
}

func (r *urlRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = r.targetHost + ":" + r.targetPort
	return http.DefaultTransport.RoundTrip(req)
}

func newTestClient(serverURL string) *apiClient {
	parts := strings.TrimPrefix(serverURL, "http://")
	hostPort := strings.SplitN(parts, ":", 2)
	host, port := hostPort[0], hostPort[1]

	return &apiClient{
		token: "test-token",
		http: &http.Client{
			Transport: &urlRewriter{targetHost: host, targetPort: port},
		},
	}
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
