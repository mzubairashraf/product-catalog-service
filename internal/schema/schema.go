// Package schema applies the DDL in migrations/ to a Spanner database.
package schema

import (
	"context"
	"fmt"
	"strings"

	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Statements splits a migration file into DDL statements, dropping comments.
func Statements(sql string) []string {
	var body strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			body.WriteString(line + "\n")
		}
	}
	var statements []string
	for _, stmt := range strings.Split(body.String(), ";") {
		if stmt = strings.TrimSpace(stmt); stmt != "" {
			statements = append(statements, stmt)
		}
	}
	return statements
}

// EnsureEmulatorInstance creates the instance on the Spanner emulator if it
// does not exist. Real instances are provisioned outside the application.
func EnsureEmulatorInstance(ctx context.Context, project, instanceID string) error {
	client, err := instance.NewInstanceAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	op, err := client.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     "projects/" + project,
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			Config:      "projects/" + project + "/instanceConfigs/emulator-config",
			DisplayName: instanceID,
			NodeCount:   1,
		},
	})
	if err == nil {
		_, err = op.Wait(ctx)
	}
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return fmt.Errorf("creating instance: %w", err)
	}
	return nil
}

// CreateDatabase creates the database with the given DDL. It reports
// created=false, with no error, if the database already exists.
func CreateDatabase(ctx context.Context, project, instanceID, databaseID string, ddl []string) (created bool, err error) {
	client, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return false, err
	}
	defer client.Close()

	op, err := client.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          "projects/" + project + "/instances/" + instanceID,
		CreateStatement: "CREATE DATABASE `" + databaseID + "`",
		ExtraStatements: ddl,
	})
	if status.Code(err) == codes.AlreadyExists {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("creating database: %w", err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("applying schema: %w", err)
	}
	return true, nil
}

// DropDatabase removes a database. It is used by tests.
func DropDatabase(ctx context.Context, name string) error {
	client, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.DropDatabase(ctx, &databasepb.DropDatabaseRequest{Database: name})
}
