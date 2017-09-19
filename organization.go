package codeship

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"

	"github.com/pkg/errors"
)

// Organization holds the configuration for the current API client scoped to the Organization. Should not
// be modified concurrently
type Organization struct {
	UUID   string
	Name   string
	Scopes []string
	client *Client
}

func (o *Organization) request(method, path string, params interface{}) ([]byte, error) {
	if o.client == nil {
		return nil, errors.New("client not instantiated")
	}

	url := o.client.baseURL + path
	// Replace nil with a JSON object if needed
	var reqBody io.Reader
	if params != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(params); err != nil {
			return nil, err
		}
		reqBody = buf
	}

	var err error

	if o.client.AuthenticationRequired() {
		if err = o.client.Authenticate(); err != nil {
			return nil, errors.Wrap(err, "authentication failed")
		}
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request creation failed")
	}

	// Apply any user-defined headers first
	req.Header = cloneHeader(o.client.headers)
	req.Header.Set("Authorization", "Bearer "+o.client.authentication.AccessToken)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if o.client.verbose {
		dumpReq, _ := httputil.DumpRequest(req, params != nil)
		o.client.logger.Println(string(dumpReq))
	}

	resp, err := o.client.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}

	if o.client.verbose {
		dumpResp, _ := httputil.DumpResponse(resp, true)
		o.client.logger.Println(string(dumpResp))
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "could not read response body")
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		break
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("HTTP status %d: invalid credentials", resp.StatusCode)
	case http.StatusForbidden:
		return nil, fmt.Errorf("HTTP status %d: insufficient permissions", resp.StatusCode)
	default:
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("HTTP status %d: server error", resp.StatusCode)
		}

		var s string
		if body != nil {
			s = string(body)
		}
		return nil, fmt.Errorf("HTTP status %d: content %q", resp.StatusCode, s)
	}

	return body, nil
}

// cloneHeader returns a shallow copy of the header.
// copied from https://godoc.org/github.com/golang/gddo/httputil/header#Copy
func cloneHeader(header http.Header) http.Header {
	h := make(http.Header)
	for k, vs := range header {
		h[k] = vs
	}
	return h
}
