package docs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bytesparadise/libasciidoc"
	"github.com/bytesparadise/libasciidoc/pkg/configuration"
)

type Service struct {
	docsDir string
	cache   map[string]string // filename -> html content
	mu      sync.RWMutex
}

func NewService(docsDir string) *Service {
	return &Service{
		docsDir: docsDir,
		cache:   make(map[string]string),
	}
}

func (s *Service) GetDoc(ctx context.Context, filename string) (string, error) {
	s.mu.RLock()
	content, ok := s.cache[filename]
	s.mu.RUnlock()
	if ok {
		return content, nil
	}

	// Read and render
	path := filepath.Join(s.docsDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read doc file: %w", err)
	}

	output := bytes.NewBuffer(nil)
	config := configuration.NewConfiguration(
		configuration.WithHeaderFooter(false), // We'll embed it in our layout
		configuration.WithAttribute("toc", "left"),
	)
	
	_, err = libasciidoc.Convert(bytes.NewReader(data), output, config)
	if err != nil {
		return "", fmt.Errorf("failed to convert asciidoc: %w", err)
	}

	html := output.String()
	
	s.mu.Lock()
	s.cache[filename] = html
	s.mu.Unlock()

	return html, nil
}

func (s *Service) ListDocs() ([]string, error) {
	entries, err := os.ReadDir(s.docsDir)
	if err != nil {
		return nil, err
	}

	var docs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".adoc") {
			docs = append(docs, entry.Name())
		}
	}
	return docs, nil
}
