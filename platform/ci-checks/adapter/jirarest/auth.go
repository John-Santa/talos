package jirarest

import (
	"encoding/base64"
	"net/http"
)

func setBasicAuth(req *http.Request, email, token string) {
	raw := email + ":" + token
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(raw)))
}
