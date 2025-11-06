package web

import (
	"html/template"
)

func parseTemplates() (*template.Template, error) {
	return template.ParseGlob("internal/web/*.html")
}
