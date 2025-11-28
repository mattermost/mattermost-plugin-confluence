package main

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	routePrefixInstance = "instance"
)

func respondErr(w http.ResponseWriter, code int, err error) (int, error) {
	http.Error(w, err.Error(), code)
	return code, err
}

func (p *Plugin) loadTemplates(dir string) (map[string]*template.Template, error) {
	dir = filepath.Clean(dir)
	templates := make(map[string]*template.Template)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		template, err := template.ParseFiles(path)
		if err != nil {
			p.errorf("OnActivate: failed to parse template %s: %v", path, err)
			return nil
		}
		key := path[len(dir):]
		templates[key] = template
		p.debugf("loaded template %s", key)
		return nil
	})
	if err != nil {
		return nil, errors.WithMessage(err, "OnActivate: failed to load templates")
	}
	return templates, nil
}

// splitInstancePath extracts the instance ID from a given route and returns the instance URL along with the remaining path
// if the route does not contain a valid instance ID, the original route is returned.
func splitInstancePath(route string) (instanceURL string, remainingPath string) {
	leadingSlash := ""
	ss := strings.Split(route, "/")

	// Remove leading slash if present
	if len(ss) > 1 && ss[0] == "" {
		leadingSlash = "/"
		ss = ss[1:]
	}

	// If there's not enough parts in the path, return the original route
	if len(ss) < 2 {
		return "", route
	}

	// Remove API version prefix if present (e.g., "api/v1")
	if ss[0] == "api" && strings.Contains(ss[1], "v") {
		ss = ss[2:]
	}

	// If the first segment is not the expected instance prefix, return the route as is
	if ss[0] != routePrefixInstance {
		return route, route
	}

	// Try to decode the instance ID
	id, err := decode(ss[1])
	if err != nil {
		return "", route
	}

	// Return the decoded instance ID and the remaining path
	return string(id), leadingSlash + strings.Join(ss[2:], "/")
}

func (p *Plugin) respondTemplate(w http.ResponseWriter, key string, r *http.Request, status int, contentType string, values interface{}) (int, error) {
	if key == "" {
		_, key = splitInstancePath(r.URL.Path) // Extract key from URL if not provided
	}

	w.Header().Set("Content-Type", contentType)
	t := p.templates[key]
	if t == nil {
		return respondErr(w, http.StatusInternalServerError, errors.New("no template found for "+key))
	}
	if err := t.Execute(w, values); err != nil {
		return http.StatusInternalServerError, errors.WithMessage(err, "failed to write response")
	}
	return status, nil
}
