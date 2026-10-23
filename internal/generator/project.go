package generator

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed all:_template
var projectTemplate embed.FS

// ProjectConfig selects the parent directory and new project name.
type ProjectConfig struct {
	Dir  string
	Name string
}

// ProjectResult reports the created project directory and file count.
type ProjectResult struct {
	Path  string
	Files int
}

// CreateProject creates the one supported MyGo web application template.
func CreateProject(ctx context.Context, config ProjectConfig) (_ ProjectResult, err error) {
	if ctx == nil {
		return ProjectResult{}, errors.New("create project: context is required")
	}
	if err := ctx.Err(); err != nil {
		return ProjectResult{}, fmt.Errorf("create project: %w", err)
	}
	name, err := parseScaffoldName(config.Name)
	if err != nil {
		return ProjectResult{}, err
	}
	parent := config.Dir
	if parent == "" {
		parent = "."
	}
	slug := strings.ReplaceAll(name.Snake, "_", "-")
	target := filepath.Join(parent, slug)
	if err := os.Mkdir(target, 0o750); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ProjectResult{}, fmt.Errorf("refuse to overwrite existing project %s", slug)
		}
		return ProjectResult{}, fmt.Errorf("create project directory %s: %w", slug, err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(target) // #nosec G703 -- target is a newly created child derived from a validated name.
		}
	}()

	title := titleFromSnake(name.Snake)
	replacer := strings.NewReplacer(
		"__PROJECT_TITLE__", title,
		"__PROJECT_SLUG__", slug,
		"__PROJECT_IDENTIFIER__", "com.example."+strings.ReplaceAll(name.Snake, "_", ""),
		"mygo-framework-notes", slug,
		"MyGo Framework Notes", title,
		"mygoframeworknotes", strings.ReplaceAll(name.Snake, "_", ""),
		"mygo-framework", slug,
	)

	files := 0
	err = fs.WalkDir(projectTemplate, "_template", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk project template: %w", walkErr)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("create project: %w", err)
		}
		relative := strings.TrimPrefix(path, "_template/")
		if path == "_template" {
			return nil
		}
		relative = strings.TrimSuffix(relative, ".tmpl")
		destination := filepath.Join(target, filepath.FromSlash(relative))
		if entry.IsDir() {
			if err := os.Mkdir(destination, 0o750); err != nil {
				return fmt.Errorf("create project directory %s: %w", relative, err)
			}
			return nil
		}
		content, err := fs.ReadFile(projectTemplate, path)
		if err != nil {
			return fmt.Errorf("read project template %s: %w", relative, err)
		}
		rendered := []byte(replacer.Replace(string(content)))
		if strings.HasSuffix(relative, ".go") {
			rendered, err = format.Source(rendered)
			if err != nil {
				return fmt.Errorf("format project file %s: %w", relative, err)
			}
		}
		if err := writeNewFile(destination, rendered); err != nil {
			return fmt.Errorf("write project file %s: %w", relative, err)
		}
		files++
		return nil
	})
	if err != nil {
		return ProjectResult{}, fmt.Errorf("create project from template: %w", err)
	}
	command := exec.CommandContext(ctx, "git", "init", "--quiet", "--initial-branch=main")
	command.Dir = target
	if output, commandErr := command.CombinedOutput(); commandErr != nil {
		return ProjectResult{}, fmt.Errorf("initialize project Git repository: %w: %s", commandErr, strings.TrimSpace(string(output)))
	}
	keep = true
	return ProjectResult{Path: target, Files: files}, nil
}

func titleFromSnake(value string) string {
	parts := strings.Split(value, "_")
	for index, part := range parts {
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
