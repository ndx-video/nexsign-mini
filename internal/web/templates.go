// Package web contains HTML templates and template helpers used by the web
// server. Templates are kept local so the web dashboard can operate offline.
// The files in this package are simple glue between static HTML assets and
// the server-side data rendered into the dashboard.
package web

import (
	"html/template"
	"os"
	"path/filepath"
)

// parseTemplates finds and parses the HTML templates.
func parseTemplates() (*template.Template, error) {
	templatePath := filepath.Join(getTemplateDir(), "*.html")
	tmpl, err := template.ParseGlob(templatePath)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

// getTemplateDir returns the template directory from environment variable or default
func getTemplateDir() string {
	if dir := os.Getenv("TEMPLATE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("internal", "web")
}
