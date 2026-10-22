package generator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FeatureConfig selects the project and feature name for the primary scaffold.
type FeatureConfig struct {
	Dir  string
	Name string
}

// FeatureResult reports hand-written feature files and regenerated framework files.
type FeatureResult struct {
	Files     []string
	Generated GenerateResult
}

type featureData struct {
	Name       string
	Snake      string
	ModulePath string
	Table      string
}

// ScaffoldFeature creates one complete feature skeleton and then regenerates adapters.
func ScaffoldFeature(ctx context.Context, config FeatureConfig) (_ FeatureResult, err error) {
	if ctx == nil {
		return FeatureResult{}, errors.New("scaffold feature: context is required")
	}
	directory := config.Dir
	if directory == "" {
		directory = "."
	}
	name, err := parseScaffoldName(config.Name)
	if err != nil {
		return FeatureResult{}, err
	}
	modulePath, err := readModuleDirective(directory)
	if err != nil {
		return FeatureResult{}, err
	}
	files, err := renderFeatureFiles(directory, featureData{
		Name:       name.Go,
		Snake:      name.Snake,
		ModulePath: modulePath,
		Table:      name.Snake + "_records",
	})
	if err != nil {
		return FeatureResult{}, err
	}

	for _, file := range files {
		// #nosec G304 -- file paths are derived from a validated name under fixed project directories.
		_, statErr := os.Lstat(filepath.Join(directory, file.path))
		if statErr == nil {
			return FeatureResult{}, fmt.Errorf("refuse to overwrite existing file %s", file.path)
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return FeatureResult{}, fmt.Errorf("inspect feature path %s: %w", file.path, statErr)
		}
	}

	created := make([]string, 0, len(files))
	defer func() {
		if err == nil {
			return
		}
		for index := len(created) - 1; index >= 0; index-- {
			_ = os.Remove(filepath.Join(directory, created[index]))
		}
	}()
	for _, file := range files {
		if err = writeNewFile(filepath.Join(directory, file.path), file.content); err != nil {
			return FeatureResult{}, fmt.Errorf("write feature file %s: %w", file.path, err)
		}
		created = append(created, file.path)
	}

	generated, err := Generate(ctx, GenerateConfig{Dir: directory})
	if err != nil {
		return FeatureResult{}, fmt.Errorf("generate feature adapters: %w", err)
	}
	return FeatureResult{Files: created, Generated: generated}, nil
}

func readModuleDirective(directory string) (string, error) {
	path := filepath.Join(directory, "go.mod")
	// #nosec G304 -- path is the fixed module file under the selected project directory.
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", errors.New("read go.mod: module directive is required")
}

func renderFeatureFiles(directory string, data featureData) ([]generatedFile, error) {
	migrationPath, err := nextMigrationPath(directory, "create_"+data.Snake+"_records")
	if err != nil {
		return nil, err
	}
	templates := []struct {
		path   string
		source string
	}{
		{path: filepath.ToSlash(filepath.Join("app", "models", data.Snake+".go")), source: featureModelTemplate},
		{path: filepath.ToSlash(filepath.Join("app", "requests", data.Snake+".go")), source: featureRequestTemplate},
		{path: filepath.ToSlash(filepath.Join("app", "services", data.Snake+".go")), source: featureServiceTemplate},
		{path: filepath.ToSlash(filepath.Join("app", "bootstrap", data.Snake+".go")), source: featureProviderTemplate},
	}
	files := make([]generatedFile, 0, len(templates)+1)
	for _, item := range templates {
		content, renderErr := renderGo(item.source, data)
		if renderErr != nil {
			return nil, fmt.Errorf("render feature file %s: %w", item.path, renderErr)
		}
		files = append(files, generatedFile{path: item.path, content: content})
	}
	migration, err := renderText(featureMigrationTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("render feature migration: %w", err)
	}
	return append(files, generatedFile{path: migrationPath, content: migration}), nil
}

func renderText(source string, data any) ([]byte, error) {
	parsed, err := templateForGeneration(source)
	if err != nil {
		return nil, err
	}
	var output strings.Builder
	if err := parsed.Execute(&output, data); err != nil {
		return nil, fmt.Errorf("execute scaffold template: %w", err)
	}
	return []byte(output.String()), nil
}

const featureModelTemplate = `package models

import "time"

// {{.Name}} is persisted application data.
type {{.Name}} struct {
	ID uint ` + "`gorm:\"primaryKey\" json:\"id\"`" + `
	Name string ` + "`gorm:\"not null\" json:\"name\"`" + `
	CreatedAt time.Time ` + "`gorm:\"not null\" json:\"createdAt\"`" + `
}

// TableName keeps the model aligned with its generated migration.
func ({{.Name}}) TableName() string { return "{{.Table}}" }
`

const featureRequestTemplate = `package requests

// Create{{.Name}}Input is validated at the service boundary.
type Create{{.Name}}Input struct {
	Name string ` + "`json:\"name\" validate:\"required\" jsonschema:\"The {{.Name}} name to store.\"`" + `
}
`

const featureServiceTemplate = `package services

import (
	"context"
	"fmt"
	"strings"

	"{{.ModulePath}}/app/models"
	"{{.ModulePath}}/app/requests"
	"{{.ModulePath}}/validation"

	"gorm.io/gorm"
)

// Create{{.Name}}Output is the result of creating one {{.Name}}.
type Create{{.Name}}Output struct {
	{{.Name}} models.{{.Name}} ` + "`json:\"{{.Snake}}\" jsonschema:\"The created {{.Name}}.\"`" + `
}

// {{.Name}} implements the {{.Name}} feature.
type {{.Name}} struct {
	database *gorm.DB
	validator *validation.Validator
}

// New{{.Name}} constructs the service from direct dependencies.
func New{{.Name}}(database *gorm.DB, validator *validation.Validator) *{{.Name}} {
	return &{{.Name}}{database: database, validator: validator}
}

// Create validates and stores one {{.Name}}.
// framework:command {{.Snake}}.create
func (s *{{.Name}}) Create(ctx context.Context, input requests.Create{{.Name}}Input) (Create{{.Name}}Output, error) {
	input.Name = strings.TrimSpace(input.Name)
	if err := s.validator.Struct(input); err != nil {
		return Create{{.Name}}Output{}, fmt.Errorf("validate create {{.Snake}} input: %w", err)
	}

	var output Create{{.Name}}Output
	if err := s.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		output.{{.Name}} = models.{{.Name}}{Name: input.Name}
		if err := transaction.Create(&output.{{.Name}}).Error; err != nil {
			return fmt.Errorf("insert {{.Snake}}: %w", err)
		}
		return nil
	}); err != nil {
		return Create{{.Name}}Output{}, fmt.Errorf("create {{.Snake}} transaction: %w", err)
	}
	return output, nil
}
`

const featureProviderTemplate = `package bootstrap

import (
	"fmt"

	"{{.ModulePath}}/app/services"
	"{{.ModulePath}}/internal/database"
	"{{.ModulePath}}/validation"

	"github.com/samber/do/v2"
)

func provide{{.Name}}(injector do.Injector) (*services.{{.Name}}, error) {
	database, err := do.Invoke[*database.Database](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve database for {{.Snake}} service: %w", err)
	}
	validator, err := do.Invoke[*validation.Validator](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve validator for {{.Snake}} service: %w", err)
	}
	return services.New{{.Name}}(database.GORM, validator), nil
}
`

const featureMigrationTemplate = `-- +goose Up
CREATE TABLE {{.Table}} (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE {{.Table}};
`
