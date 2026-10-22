// Package main provides the non-GUI database foundation probe.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"mygo-framework/app/models"
	"mygo-framework/database/migrations"

	appdb "mygo-framework/internal/database"
)

func main() {
	path := flag.String("database", "", "SQLite database file to probe")
	body := flag.String("note", "database probe", "note body to store and reload")
	flag.Parse()

	if *path == "" {
		log.Fatal("-database is required")
	}

	ctx := context.Background()
	database, err := appdb.Open(ctx, *path, migrations.FS())
	if err != nil {
		log.Fatal(err)
	}

	note := models.Note{Body: *body}
	if err := database.GORM.WithContext(ctx).Create(&note).Error; err != nil {
		_ = database.Close()
		log.Fatalf("store probe note: %v", err)
	}
	if err := database.Close(); err != nil {
		log.Fatalf("close database after store: %v", err)
	}

	database, err = appdb.Open(ctx, *path, migrations.FS())
	if err != nil {
		log.Fatalf("reopen database: %v", err)
	}

	var reloaded models.Note
	reloadErr := database.GORM.WithContext(ctx).First(&reloaded, note.ID).Error
	closeErr := database.Close()
	if reloadErr != nil {
		log.Fatalf("reload probe note: %v", reloadErr)
	}
	if closeErr != nil {
		log.Fatalf("close database after reload: %v", closeErr)
	}

	fmt.Printf("stored and reloaded note %d: %s\n", reloaded.ID, reloaded.Body)
}
