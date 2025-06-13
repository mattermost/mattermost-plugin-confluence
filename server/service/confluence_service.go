package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

const ErrorStatusNotFound = "No content found"

type ConfluenceStatus struct {
	State string `json:"state"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func NormalizeConfluenceURL(confluenceURL string) (string, error) {
	u, err := url.Parse(confluenceURL)
	if err != nil {
		return "", fmt.Errorf("could not parse Confluence url: %w", err)
	}

	// If the parsed URL does not contain a host, trying to extract the host from the path
	if u.Host == "" {
		ss := strings.Split(u.Path, "/")
		if len(ss) > 0 && ss[0] != "" {
			u.Host = ss[0]
			u.Path = path.Join(ss[1:]...)
		}
		u, err = url.Parse(u.String())
		if err != nil {
			return "", err
		}
	}

	// If the URL still lacks a hostname, return an error
	if u.Host == "" {
		return "", errors.Errorf("Invalid URL, no hostname: %q", confluenceURL)
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	confluenceURL = strings.TrimSuffix(u.String(), "/")
	return confluenceURL, nil
}

// CheckConfluenceURL checks if the `/status` endpoint of the Confluence URL is accessible
// and responding with the correct state which is "RUNNING"
func CheckConfluenceURL(mattermostSiteURL, confluenceURL string, _ bool) (string, error) {
	confluenceURL, err := NormalizeConfluenceURL(confluenceURL)
	if err != nil {
		config.Mattermost.LogError("Error normalizing Confluence URL", "ConfluenceURL", confluenceURL, "error", err.Error())
		return "", fmt.Errorf("unable to normalize Confluence url. Confluence URL %s. %w", confluenceURL, err)
	}

	if confluenceURL == strings.TrimSuffix(mattermostSiteURL, "/") {
		return "", fmt.Errorf("%s is the Mattermost site URL. Please use the Confluence URL", confluenceURL)
	}

	var status ConfluenceStatus
	if _, statusCode, err := CallJSON(confluenceURL, http.MethodGet, "/status", nil, &status, &http.Client{}); err != nil {
		config.Mattermost.LogError("Error making call to get Confluence server status", "Confluence URL", confluenceURL, "StatusCode", statusCode, err.Error())
		return "", fmt.Errorf("error making call to get Confluence server status. Confluence URL: %s. StatusCode:  %d, %w", confluenceURL, statusCode, err)
	}

	if status.State != "RUNNING" {
		return "", fmt.Errorf("confluence server is not in correct state, it should be up and running: %q", confluenceURL)
	}

	return confluenceURL, nil
}

func CallJSONWithURL(instanceURL, path, method string, in, out interface{}, httpClient *http.Client) (responseData []byte, statusCode int, err error) {
	urlPath, err := GetEndpointURL(instanceURL, path)
	if err != nil {
		return nil, 0, err
	}

	return CallJSON(instanceURL, method, urlPath, in, out, httpClient)
}

func GetEndpointURL(instanceURL, path string) (string, error) {
	endpointURL, err := url.Parse(strings.TrimSpace(fmt.Sprintf("%s%s", instanceURL, path)))
	if err != nil {
		config.Mattermost.LogError("Error parsing URL", "Instance URL", instanceURL, "Path", path, err.Error())
		return "", err
	}

	return endpointURL.String(), nil
}

func CallJSON(url, method, path string, in, out interface{}, httpClient *http.Client) (responseData []byte, statusCode int, err error) {
	contentType := "application/json"
	buf := &bytes.Buffer{}
	if err = json.NewEncoder(buf).Encode(in); err != nil {
		return nil, 0, err
	}

	return call(url, method, path, contentType, buf, out, httpClient)
}

func call(basePath, method, path, contentType string, inBody io.Reader, out interface{}, httpClient *http.Client) (responseData []byte, statusCode int, err error) {
	pathURL, err := url.Parse(path)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to parse request path")
	}

	if pathURL.Scheme == "" || pathURL.Host == "" {
		var baseURL *url.URL
		baseURL, err = url.Parse(basePath)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to parse base URL")
		}
		pathURL = baseURL.ResolveReference(pathURL)
	}

	req, err := http.NewRequest(method, pathURL.String(), inBody)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to create request")
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	statusCode = resp.StatusCode

	responseData, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, statusCode, errors.Wrap(err, "failed to read response body")
	}

	if statusCode == http.StatusOK || statusCode == http.StatusCreated {
		if out != nil {
			if err := json.Unmarshal(responseData, out); err != nil {
				return responseData, statusCode, errors.Wrap(err, "failed to parse response JSON")
			}
		}
		return responseData, statusCode, nil
	}

	if statusCode == http.StatusNoContent {
		return nil, statusCode, nil
	}

	errResp := ErrorResponse{}
	if json.Unmarshal(responseData, &errResp) == nil && errResp.Message != "" {
		return nil, statusCode, errors.New(errResp.Message)
	}

	return nil, statusCode, errors.Errorf("unexpected response status: %d", statusCode)
}
