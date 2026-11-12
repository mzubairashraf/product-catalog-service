// Command migrate creates the database and applies migrations/. Against the
// emulator (SPANNER_EMULATOR_HOST set) it also creates the instance, so a
// fresh emulator needs no gcloud commands.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/mzubairashraf/product-catalog-service/internal/schema"
)

var databasePath = regexp.MustCompile(`^projects/([^/]+)/instances/([^/]+)/databases/([^/]+)$`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	m := databasePath.FindStringSubmatch(os.Getenv("SPANNER_DATABASE"))
	if m == nil {
		return errors.New("SPANNER_DATABASE must look like projects/P/instances/I/databases/D")
	}
	project, instanceID, databaseID := m[1], m[2], m[3]

	file := "migrations/001_initial_schema.sql"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if os.Getenv("SPANNER_EMULATOR_HOST") != "" {
		if err := schema.EnsureEmulatorInstance(ctx, project, instanceID); err != nil {
			return err
		}
	}
	created, err := schema.CreateDatabase(ctx, project, instanceID, databaseID, schema.Statements(string(raw)))
	if err != nil {
		return err
	}
	if created {
		fmt.Println("created", os.Getenv("SPANNER_DATABASE"))
	} else {
		fmt.Println("already exists:", os.Getenv("SPANNER_DATABASE"))
	}
	return nil
}
