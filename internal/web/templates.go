package web

import (
	"html/template"
	"path/filepath"
)

// parseTemplates finds and parses the HTML templates.
// For now, we only have one, but this is scalable.
func parseTemplates() (*template.Template, error) {
	// In a real application, you would embed the templates into the binary.
	// For development, we'll read them from the filesystem.
	templatePath := filepath.Join("internal", "web", "*.html")

	tmpl, err := template.ParseGlob(templatePath)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}