package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	html "github.com/levigross/exp-html"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

const (
	Script              = "script"
	Style               = "style"
	ErrorStatusNotFound = "No content found"
	Someone             = "Someone"
)

// GetKeyHash can be used to create a hash from a string
func GetKeyHash(key string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(key))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func SplitArgs(s string) ([]string, error) {
	indexes := regexp.MustCompile("\"").FindAllStringIndex(s, -1)
	if len(indexes)%2 != 0 {
		return []string{}, errors.New("quotes not closed")
	}

	indexes = append([][]int{{0, 0}}, indexes...)

	if indexes[len(indexes)-1][1] < len(s) {
		indexes = append(indexes, [][]int{{len(s), 0}}...)
	}

	var args []string
	for i := 0; i < len(indexes)-1; i++ {
		start := indexes[i][1]
		end := Min(len(s), indexes[i+1][0])

		if i%2 == 0 {
			args = append(args, strings.Split(strings.Trim(s[start:end], " "), " ")...)
		} else {
			args = append(args, s[start:end])
		}
	}

	cleanedArgs := make([]string, len(args))
	count := 0

	for _, arg := range args {
		if arg != "" {
			cleanedArgs[count] = strings.TrimSpace(arg)
			count++
		}
	}

	return cleanedArgs[0:count], nil
}

func IsSystemAdmin(userID string) bool {
	user, appErr := config.Mattermost.GetUser(userID)
	if appErr != nil {
		return false
	}
	return user.IsInRole(model.SystemAdminRoleId)
}

func Deduplicate(a []string) []string {
	check := make(map[string]int)
	result := make([]string, 0)
	for _, val := range a {
		check[val] = 1
	}

	for key := range check {
		result = append(result, key)
	}

	return result
}

func IsConfluenceCloudURL(confluenceURL string) bool {
	u, err := url.Parse(confluenceURL)
	if err != nil {
		return false
	}
	return strings.HasSuffix(u.Hostname(), ".atlassian.net")
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
	err = json.NewEncoder(buf).Encode(in)
	if err != nil {
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

	return handleResponse(resp, out)
}

func handleResponse(resp *http.Response, out interface{}) ([]byte, error) {
	if resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()

	responseData, err := ioutil.ReadAll(resp.Body)
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

	type ErrorResponse struct {
		Message string `json:"message"`
	}
	errResp := ErrorResponse{}
	if err = json.Unmarshal(responseData, &errResp); err != nil {
		return nil, err
	}
	return responseData, errors.New(errResp.Message)
}

func GetBodyForExcerpt(htmlBodyValue string) string {
	var str string
	domDocTest := html.NewTokenizer(strings.NewReader(htmlBodyValue))
	previousStartTokenTest := domDocTest.Token()
loopDomTest:
	for {
		tt := domDocTest.Next()
		switch {
		case tt == html.ErrorToken:
			break loopDomTest // End of the document,  done
		case tt == html.StartTagToken:
			previousStartTokenTest = domDocTest.Token()
		case tt == html.TextToken:
			if previousStartTokenTest.Data == Script || previousStartTokenTest.Data == Style {
				continue
			}
			TextContent := strings.TrimSpace(html.UnescapeString(string(domDocTest.Text())))
			if len(TextContent) > 0 {
				str = fmt.Sprintf("%s\n%s", str, TextContent)
			}
		}
	}
	return str
}

func CreateConfluenceURL(selfEventURL string) (string, error) {
	url, err := url.Parse(selfEventURL)
	if err != nil {
		return "", nil
	}
	return url.Scheme + "://" + url.Host, nil
}

func GetUsernameOrAnonymousName(username string) string {
	if username == "" {
		return "Someone"
	}
	return username
}

func Map(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

// IsValidURL checks if a given URL is a valid URL with a host and a http or https scheme.
func IsValidURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url schema must either be http or https")
	}

	if u.Host == "" {
		return errors.New("url must contain a host")
	}

	return nil
}
