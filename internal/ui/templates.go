package ui

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
)

type TemplateManager struct {
	cache map[string]*template.Template
}

func NewTemplateManager() (*TemplateManager, error) {
	cache := map[string]*template.Template{}

	entries, err := TemplatesFS.ReadDir("templates")
	if err != nil {
		return nil, err
	}

	var templateFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".html" {
			templateFiles = append(templateFiles, "templates/"+entry.Name())
		}
	}

	if len(templateFiles) == 0 {
		return nil, fmt.Errorf("no template files found in embedded FS")
	}

	ts, err := template.New("").ParseFS(TemplatesFS, templateFiles...)
	if err != nil {
		return nil, err
	}

	for _, page := range templateFiles {
		name := filepath.Base(page)
		name = name[:len(name)-len(filepath.Ext(name))] // strip .html
		cache[name] = ts
	}

	return &TemplateManager{cache: cache}, nil
}

func (tm *TemplateManager) Render(w io.Writer, _ string, data any) error {
	ts, ok := tm.cache["base"]
	if !ok {
		return fmt.Errorf("the template 'base' does not exist")
	}
	err := ts.ExecuteTemplate(w, "base", data)
	if err != nil {
		fmt.Printf("Error executing template: %v\n", err)
	}
	return err
}

func (tm *TemplateManager) Get(name string) (*template.Template, error) {
	ts, ok := tm.cache[name]
	if !ok {
		return nil, fmt.Errorf("template %s not found", name)
	}
	return ts, nil
}
