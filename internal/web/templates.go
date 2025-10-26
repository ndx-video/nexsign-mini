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
	templatePath := "."

	tmpl, err := template.ParseFiles(filepath.Join(templatePath, "internal", "web", "index.html"))
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}