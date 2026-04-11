package scaffold

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

// TemplateData holds data passed to templates during rendering.
type TemplateData struct {
	// DomainName is the singular lowercase name (e.g., "order")
	DomainName string

	// DomainNamePlural is the plural lowercase name (e.g., "orders")
	DomainNamePlural string

	// DomainNamePascal is PascalCase (e.g., "Order")
	DomainNamePascal string

	// DomainNameCamel is camelCase (e.g., "order")
	DomainNameCamel string

	// DomainNameSnake is snake_case (e.g., "order_item")
	DomainNameSnake string

	// ModulePath is the Go module path
	ModulePath string

	// Config holds the full scaffold configuration
	Config Config
}

// NewTemplateData creates TemplateData from a domain name and config.
func NewTemplateData(domainName string, cfg Config) TemplateData {
	return TemplateData{
		DomainName:       ToSnakeCase(domainName),
		DomainNamePlural: ToPlural(ToSnakeCase(domainName)),
		DomainNamePascal: ToPascalCase(domainName),
		DomainNameCamel:  ToCamelCase(domainName),
		DomainNameSnake:  ToSnakeCase(domainName),
		ModulePath:       cfg.ModulePath,
		Config:           cfg,
	}
}

// RenderTemplate renders a single template string with the given data.
func RenderTemplate(tmplContent string, data TemplateData) (string, error) {
	t, parseErr := template.New("scaffold").Funcs(TemplateFuncs()).Parse(tmplContent)
	if parseErr != nil {
		return "", parseErr
	}

	var buf bytes.Buffer
	execErr := t.Execute(&buf, data)
	if execErr != nil {
		return "", execErr
	}

	return buf.String(), nil
}

// RenderTemplateFile renders a template file and writes the result to outputPath.
func RenderTemplateFile(tmplContent string, data TemplateData, outputPath string) error {
	rendered, renderErr := RenderTemplate(tmplContent, data)
	if renderErr != nil {
		return renderErr
	}

	dirPath := filepath.Dir(outputPath)
	if mkdirErr := os.MkdirAll(dirPath, 0o750); mkdirErr != nil {
		return mkdirErr
	}

	return os.WriteFile(outputPath, []byte(rendered), 0o600)
}

// RenderFS renders all .tmpl files from an fs.FS, stripping the .tmpl extension,
// replacing template variables in both content and file paths.
func RenderFS(templates fs.FS, data TemplateData, outputDir string) error {
	return fs.WalkDir(templates, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		content, readErr := fs.ReadFile(templates, path)
		if readErr != nil {
			return readErr
		}

		// Strip .tmpl extension
		outPath := path
		if filepath.Ext(path) == ".tmpl" {
			outPath = outPath[:len(outPath)-5]
		}

		// Replace domain name placeholder in path
		outPath = filepath.Join(outputDir, outPath)

		return RenderTemplateFile(string(content), data, outPath)
	})
}
