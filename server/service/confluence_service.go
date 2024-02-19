package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"
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
		return "", err
	}

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
func CheckConfluenceURL(mattermostSiteURL, confluenceURL string, requireHTTPS bool) (_ string, err error) {
	confluenceURL, err = NormalizeConfluenceURL(confluenceURL)
	if err != nil {
		return "", err
	}

	if confluenceURL == strings.TrimSuffix(mattermostSiteURL, "/") {
		return "", errors.Errorf("%s is the Mattermost site URL. Please use your Confluence URL", confluenceURL)
	}

	defer func() {
		if err != nil {
			err = errors.Wrap(err, "we couldn't validate the connection to your Confluence server. "+
				"This could be because of existing firewall or proxy rules, or because the URL was entered incorrectly")
		}
	}()

	var status ConfluenceStatus
	if _, err = CallJSON(confluenceURL, http.MethodGet, "/status", nil, &status, &http.Client{}); err != nil {
		return "", err
	}

	if status.State != "RUNNING" {
		return "", errors.Errorf("Confluence server is not in correct state, it should be up and running: %q", confluenceURL)
	}

	return confluenceURL, nil
}

func CallJSONWithURL(instanceURL, path, method string, in, out interface{}, httpClient *http.Client) (responseData []byte, err error) {
	urlPath, err := GetEndpointURL(instanceURL, path)
	if err != nil {
		return nil, err
	}

	return CallJSON(instanceURL, method, urlPath, in, out, httpClient)
}

func GetEndpointURL(instanceURL, path string) (string, error) {
	endpointURL, err := url.Parse(strings.TrimSpace(fmt.Sprintf("%s%s", instanceURL, path)))
	if err != nil {
		return "", err
	}

	return endpointURL.String(), nil
}

func CallJSON(url, method, path string, in, out interface{}, httpClient *http.Client) (responseData []byte, err error) {
	contentType := "application/json"
	buf := &bytes.Buffer{}
	if err = json.NewEncoder(buf).Encode(in); err != nil {
		return nil, err
	}

	return call(url, method, path, contentType, buf, out, httpClient)
}

func call(basePath, method, path, contentType string, inBody io.Reader, out interface{}, httpClient *http.Client) (responseData []byte, err error) {
	errContext := fmt.Sprintf("confluence: Call failed: method:%s, path:%s", method, path)
	pathURL, err := url.Parse(path)
	if err != nil {
		return nil, errors.WithMessage(err, errContext)
	}

	if pathURL.Scheme == "" || pathURL.Host == "" {
		var baseURL *url.URL
		baseURL, err = url.Parse(basePath)
		if err != nil {
			return nil, errors.WithMessage(err, errContext)
		}
		if path[0] != '/' {
			path = "/" + path
		}
		path = baseURL.String() + path
	}

	req, err := http.NewRequest(method, path, inBody)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Add("Content-Type", contentType)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()

	responseData, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		if out != nil {
			err = json.Unmarshal(responseData, out)
			if err != nil {
				return responseData, err
			}
		}
		return responseData, nil

	case http.StatusNoContent:
		return nil, nil

	case http.StatusNotFound:
		return nil, errors.Errorf(ErrorStatusNotFound)
	}

	errResp := ErrorResponse{}
	if err = json.Unmarshal(responseData, &errResp); err != nil {
		return nil, err
	}

	return responseData, errors.New(errResp.Message)
}
