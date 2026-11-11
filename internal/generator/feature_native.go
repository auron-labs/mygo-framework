package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isNativeProject reports whether directory is a native starter, which has
// mygo.json and no web frontend; the CLI uses the same marker to skip web
// adapter generation.
func isNativeProject(directory string) (bool, error) {
	_, err := os.Stat(filepath.Join(directory, "mygo.json"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect MyGo configuration: %w", err)
	}
	return true, nil
}

func renderNativeFeatureFiles(directory string, data featureData) ([]generatedFile, error) {
	migrationPath, err := nextMigrationPath(directory, "create_"+data.Snake+"_records")
	if err != nil {
		return nil, err
	}
	data.Var = strings.ToLower(data.Name[:1]) + data.Name[1:]
	data.Title = titleFromSnake(data.Snake)
	templates := []struct {
		path   string
		source string
	}{
		{path: data.Snake + ".go", source: nativeFeatureTemplate},
		{path: data.Snake + "_test.go", source: nativeFeatureTestTemplate},
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

const nativeFeatureTemplate = `package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"gorm.io/gorm"
)

// {{.Name}} is one stored record.
type {{.Name}} struct {
	ID        uint      ` + "`gorm:\"primaryKey\"`" + `
	Name      string    ` + "`gorm:\"not null\"`" + `
	CreatedAt time.Time ` + "`gorm:\"not null\"`" + `
}

// TableName keeps the model aligned with its migration.
func ({{.Name}}) TableName() string { return "{{.Table}}" }

// {{.Name}}Store persists {{.Name}} records.
type {{.Name}}Store struct {
	database *gorm.DB
}

func new{{.Name}}Store(database *gorm.DB) *{{.Name}}Store {
	return &{{.Name}}Store{database: database}
}

// Create stores one record with a trimmed, non-empty name.
func (s *{{.Name}}Store) Create(ctx context.Context, name string) ({{.Name}}, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return {{.Name}}{}, errors.New("name is required")
	}
	record := {{.Name}}{Name: name}
	if err := s.database.WithContext(ctx).Create(&record).Error; err != nil {
		return {{.Name}}{}, fmt.Errorf("insert {{.Snake}}: %w", err)
	}
	return record, nil
}

// List returns the records in creation order.
func (s *{{.Name}}Store) List(ctx context.Context) ([]{{.Name}}, error) {
	var records []{{.Name}}
	if err := s.database.WithContext(ctx).Order("id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list {{.Snake}}: %w", err)
	}
	return records, nil
}

// {{.Var}}View is the retained state of the {{.Name}} list. The view only reads
// this state; the store is used from the actions that button callbacks run.
type {{.Var}}View struct {
	store   *{{.Name}}Store
	records []{{.Name}}
	draft   string
	err     string
}

// load replaces the records with the stored ones.
func (v *{{.Var}}View) load(ctx context.Context) error {
	records, err := v.store.List(ctx)
	if err != nil {
		return err
	}
	v.records = records
	return nil
}

func (v *{{.Var}}View) add() {
	ctx := context.Background()
	if _, err := v.store.Create(ctx, v.draft); err != nil {
		log.Printf("add {{.Snake}}: %v", err)
		v.err = "Could not add that. Enter a name and try again."
		return
	}
	if err := v.load(ctx); err != nil {
		log.Printf("reload {{.Snake}}: %v", err)
		v.err = "Could not refresh the list."
		return
	}
	v.draft, v.err = "", ""
}

func (v *{{.Var}}View) view(c *ui.Context) {
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		components.Text(c, "{{.Title}}").FontSize(22).Bold()
		components.TextBox(c, &v.draft, components.TextBoxOptions{}).Label("New {{.Title}} name")
		components.Button(c, "Add", components.ButtonPrimary).OnClick(v.add)
		if v.err != "" {
			components.Text(c, v.err).Wrap()
		}
		if len(v.records) == 0 {
			components.Text(c, "Nothing here yet.").Wrap()
		}
		for _, record := range v.records {
			components.Text(c.Key(record.ID), record.Name).Wrap()
		}
	})
}
`

const nativeFeatureTestTemplate = `package main

import (
	"path/filepath"
	"testing"

	"{{.ModulePath}}/database/migrations"
	"{{.FrameworkModule}}/database"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func open{{.Name}}TestStore(t *testing.T) *{{.Name}}Store {
	t.Helper()
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"), migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return new{{.Name}}Store(db.GORM)
}

func Test{{.Name}}StoreCreatesAndListsInOrder(t *testing.T) {
	store := open{{.Name}}TestStore(t)
	for _, name := range []string{"  first ", "second"} {
		if _, err := store.Create(t.Context(), name); err != nil {
			t.Fatal(err)
		}
	}
	records, err := store.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Name != "first" || records[1].Name != "second" {
		t.Fatalf("records = %+v", records)
	}
	if _, err := store.Create(t.Context(), "   "); err == nil {
		t.Fatal("a blank name was stored")
	}
}

func Test{{.Name}}ViewAddsARecord(t *testing.T) {
	v := &{{.Var}}View{store: open{{.Name}}TestStore(t), draft: "Example"}
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); v.view(c) }, 640, 480)
	tt.Frame()
	if err := tt.Click("Add"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Example") || v.draft != "" {
		t.Fatalf("view = %q, draft = %q", tt.Texts(), v.draft)
	}
}
`
