package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"platform-service/internal/config"
	"platform-service/internal/migration"
	"platform-service/internal/storage"
)

func main() {
	configName := flag.String("config", "config.local", "config file name without .yaml or with .yaml")
	command := flag.String("command", "status", "migration command: status or up")
	flag.Parse()

	cfg, err := config.Load(*configName)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := storage.ConnectDB(cfg.Database)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	switch *command {
	case "status":
		items, err := migration.ListStatus(db)
		if err != nil {
			log.Fatalf("migration status: %v", err)
		}
		for _, item := range items {
			state := "pending"
			if item.Applied {
				state = "applied"
			}
			appliedAt := ""
			if item.AppliedAt != nil {
				appliedAt = item.AppliedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			fmt.Printf("%d\t%s\t%s\t%s\n", item.Version, item.Name, state, appliedAt)
		}
	case "up":
		if err := migration.Up(db); err != nil {
			log.Fatalf("migration up: %v", err)
		}
		version, err := migration.CurrentVersion(db)
		if err != nil {
			log.Fatalf("migration current version: %v", err)
		}
		fmt.Printf("migration_up_ok current_version=%d\n", version)
	default:
		fmt.Fprintf(os.Stderr, "unsupported command %q; use status or up\n", *command)
		os.Exit(2)
	}
}
