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
