/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/errors"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/telemetry"
)

// CreateOrUpdateDatabase creates or updates a PostgreSQL database
func CreateOrUpdateDatabase(
	ctx context.Context, host string, port int32,
	adminUser, adminPassword, sslMode,
	databaseName, owner, schemaName, templateDatabase string,
) error {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.CreateOrUpdateDatabase",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String(telemetry.AttrDatabase, databaseName),
			attribute.String(telemetry.AttrOperation,
				"create_or_update_database"),
		))
	defer span.End()
	_ = ctx // ctx is used for span propagation
	// Connect to PostgreSQL as admin user
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, DefaultDB, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Escape database name and owner for SQL identifiers (PostgreSQL uses double quotes)
	// Replace double quotes with two double quotes to escape them
	escapedDatabaseName := pq.QuoteIdentifier(databaseName)
	escapedOwner := pq.QuoteIdentifier(owner)

	// Check if database exists
	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).
			WithOp("CreateOrUpdateDatabase.checkDatabaseExists")
	}

	if exists {
		// Update database owner
		query := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", escapedDatabaseName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).
				WithOp("CreateOrUpdateDatabase.updateDatabaseOwner")
		}
		return nil
	}

	// Create new database
	var query string
	if templateDatabase != "" {
		// Create database from template
		escapedTemplate := pq.QuoteIdentifier(templateDatabase)
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s TEMPLATE %s",
			escapedDatabaseName, escapedOwner, escapedTemplate)
	} else {
		// Create database normally
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s", escapedDatabaseName, escapedOwner)
	}
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateDatabase.createDatabase")
	}

	// Create schema in the database if specified (and not "public" which is created by default)
	if schemaName == "" || schemaName == "public" {
		return nil
	}

	// Connect to the newly created database
	connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, databaseName, sslMode)

	dbSchema, err := sql.Open("postgres", connStr)
	if err != nil {
		op := fmt.Sprintf("CreateOrUpdateDatabase.sql.Open(%s)", databaseName)
		return operrors.Wrap(operrors.ErrPostgresqlConnectionFailed, err).WithOp(op)
	}
	defer dbSchema.Close()

	// Escape schema name for SQL identifier
	escapedSchemaName := pq.QuoteIdentifier(schemaName)

	// Check if schema exists
	var schemaExists bool
	err = dbSchema.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schemaName).Scan(&schemaExists)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).
			WithOp("CreateOrUpdateDatabase.checkSchemaExists")
	}

	if !schemaExists {
		return nil
	}

	// Update schema owner
	query = fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
	_, err = dbSchema.ExecContext(connCtx, query)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).
			WithOp("CreateOrUpdateDatabase.updateSchemaOwner")
	}

	return nil
}

// CreateOrUpdateUser creates or updates a PostgreSQL user
func CreateOrUpdateUser(
	ctx context.Context, host string, port int32,
	adminUser, adminPassword, sslMode, username, password string,
) error {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.CreateOrUpdateUser",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String(telemetry.AttrUsername, username),
			attribute.String(telemetry.AttrOperation,
				"create_or_update_user"),
		))
	defer span.End()
	_ = ctx // ctx is used for span propagation
	// Connect to PostgreSQL as admin user
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, DefaultDB, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlConnectionFailed, err).WithOp("CreateOrUpdateUser.sql.Open")
	}
	defer db.Close()

	// Escape username for SQL identifier (PostgreSQL uses double quotes)
	// Replace double quotes with two double quotes to escape them
	escapedUsername := pq.QuoteIdentifier(username)

	// Escape password for SQL string literal (replace single quotes with two single quotes)
	escapedPassword := pq.QuoteLiteral(password)

	// Check if user exists
	var exists bool
	err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateUser.checkUserExists")
	}

	if exists {
		// Update user password - PostgreSQL doesn't support parameters in ALTER USER
		query := fmt.Sprintf("ALTER USER %s WITH PASSWORD %s", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).
				WithOp("CreateOrUpdateUser.updateUserPassword")
		}
	} else {
		// Create new user - PostgreSQL doesn't support parameters in CREATE USER
		query := fmt.Sprintf("CREATE USER %s WITH PASSWORD %s", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateUser.createUser")
		}
	}

	return nil
}

// getCurrentMembers retrieves the current members of a group role
func getCurrentMembers(
	ctx context.Context, db *sql.DB, groupRole string,
) (map[string]bool, error) {
	currentMembers := make(map[string]bool)
	rows, err := db.QueryContext(ctx, `
		SELECT r.rolname 
		FROM pg_roles r 
		JOIN pg_auth_members m ON r.oid = m.member 
		JOIN pg_roles g ON g.oid = m.roleid 
		WHERE g.rolname = $1`, groupRole)
	if err != nil {
		return nil, operrors.Wrap(
			operrors.ErrPostgresqlOperationFailed, err,
		).WithOp("CreateOrUpdateRoleGroup.queryCurrentMembers")
	}
	defer rows.Close()

	for rows.Next() {
		var member string
		if err := rows.Scan(&member); err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		currentMembers[member] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating members: %w", err)
	}
	return currentMembers, nil
}

// syncRoleGroupMembers adds and removes members to match desired state
func syncRoleGroupMembers(
	ctx context.Context, db *sql.DB,
	escapedGroupRole string,
	currentMembers map[string]bool,
	memberRoles []string,
) error {
	desiredMembers := make(map[string]bool, len(memberRoles))
	for _, memberRole := range memberRoles {
		desiredMembers[memberRole] = true
		if currentMembers[memberRole] {
			continue
		}

		var memberExists bool
		err := db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)",
			memberRole).Scan(&memberExists)
		if err != nil {
			return operrors.Wrap(
				operrors.ErrPostgresqlOperationFailed, err,
			).WithOp("CreateOrUpdateRoleGroup.checkMemberRoleExists")
		}
		if !memberExists {
			continue
		}

		escapedMemberRole := pq.QuoteIdentifier(memberRole)
		query := fmt.Sprintf(
			"GRANT %s TO %s", escapedGroupRole, escapedMemberRole,
		)
		if _, err = db.ExecContext(ctx, query); err != nil {
			op := fmt.Sprintf(
				"CreateOrUpdateRoleGroup.addMemberToGroup(%s)",
				memberRole)
			return operrors.Wrap(
				operrors.ErrPostgresqlOperationFailed, err,
			).WithOp(op)
		}
	}

	for member := range currentMembers {
		if desiredMembers[member] {
			continue
		}
		escapedMemberRole := pq.QuoteIdentifier(member)
		// PostgreSQL REVOKE cannot use parameterized queries for role names;
		// inputs are escaped via pq.QuoteIdentifier
		query := fmt.Sprintf( //nolint:gosec // G201: safe - escaped via pq.QuoteIdentifier
			"REVOKE %s FROM %s", escapedGroupRole, escapedMemberRole,
		)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf(
				"failed to remove member %s from group role: %w",
				member, err)
		}
	}

	return nil
}

// CreateOrUpdateRoleGroup creates or updates a PostgreSQL role group
func CreateOrUpdateRoleGroup(
	ctx context.Context, host string, port int32,
	adminUser, adminPassword, sslMode,
	groupRole string, memberRoles []string,
) error {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.CreateOrUpdateRoleGroup",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.role_group", groupRole),
			attribute.String(telemetry.AttrOperation,
				"create_or_update_role_group"),
		))
	defer span.End()
	_ = ctx // ctx is used for span propagation
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, DefaultDB, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return operrors.Wrap(
			operrors.ErrPostgresqlConnectionFailed, err,
		).WithOp("CreateOrUpdateRoleGroup.sql.Open")
	}
	defer db.Close()

	escapedGroupRole := pq.QuoteIdentifier(groupRole)

	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)",
		groupRole).Scan(&exists)
	if err != nil {
		return operrors.Wrap(
			operrors.ErrPostgresqlOperationFailed, err,
		).WithOp("CreateOrUpdateRoleGroup.checkGroupRoleExists")
	}

	if !exists {
		query := fmt.Sprintf("CREATE ROLE %s", escapedGroupRole)
		if _, err = db.ExecContext(connCtx, query); err != nil {
			return operrors.Wrap(
				operrors.ErrPostgresqlOperationFailed, err,
			).WithOp("CreateOrUpdateRoleGroup.createGroupRole")
		}
	}

	currentMembers, err := getCurrentMembers(
		connCtx, db, groupRole,
	)
	if err != nil {
		return err
	}

	return syncRoleGroupMembers(
		connCtx, db, escapedGroupRole, currentMembers, memberRoles,
	)
}

// CreateOrUpdateSchema creates or updates a PostgreSQL schema
func CreateOrUpdateSchema(
	ctx context.Context, host string, port int32,
	adminUser, adminPassword, sslMode,
	databaseName, schemaName, owner string,
) error {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.CreateOrUpdateSchema",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String(telemetry.AttrDatabase, databaseName),
			attribute.String("db.schema", schemaName),
			attribute.String(telemetry.AttrOperation,
				"create_or_update_schema"),
		))
	defer span.End()
	_ = ctx // ctx is used for span propagation
	// Connect to the specific database
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, databaseName, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Escape schema name and owner for SQL identifiers (PostgreSQL uses double quotes)
	escapedSchemaName := pq.QuoteIdentifier(schemaName)
	escapedOwner := pq.QuoteIdentifier(owner)

	// Check if schema exists
	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schemaName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if schema exists: %w", err)
	}

	if exists {
		// Update schema owner
		query := fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update schema owner: %w", err)
		}
	} else {
		// Create new schema with owner
		query := fmt.Sprintf("CREATE SCHEMA %s AUTHORIZATION %s", escapedSchemaName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
	}

	return nil
}

// applyDatabaseGrants applies database-level grants from the postgres database
func applyDatabaseGrants(
	ctx context.Context,
	host string, port int32,
	adminUser, adminPassword, sslMode string,
	escapedDBName, escapedRole string,
	databaseGrants []instancev1alpha1.GrantItem,
) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, DefaultDB, sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection to %s: %w", DefaultDB, err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping postgres database: %w", err)
	}

	for _, grant := range databaseGrants {
		privilegesStr := strings.Join(grant.Privileges, ", ")
		query := fmt.Sprintf(
			"GRANT %s ON DATABASE %s TO %s",
			privilegesStr, escapedDBName, escapedRole)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to apply database grant: %w", err)
		}
	}
	return nil
}

// applyNonDatabaseGrants applies schema/table/sequence/function grants
// and default privileges from the target database
func applyNonDatabaseGrants(
	ctx context.Context,
	host string, port int32,
	adminUser, adminPassword, sslMode, databaseName string,
	escapedRole string,
	otherGrants []instancev1alpha1.GrantItem,
	defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem,
) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, databaseName, sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf(
			"failed to open database connection to %s: %w",
			databaseName, err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf(
			"failed to ping database %s: %w", databaseName, err)
	}

	for _, grant := range otherGrants {
		if err := applyGrantItem(ctx, db, escapedRole, grant); err != nil {
			return fmt.Errorf(
				"failed to apply grant %s: %w", grant.Type, err)
		}
	}

	for _, defaultPriv := range defaultPrivileges {
		if err := applyDefaultPrivilege(
			ctx, db, escapedRole, defaultPriv,
		); err != nil {
			return fmt.Errorf(
				"failed to apply default privilege for %s: %w",
				defaultPriv.ObjectType, err)
		}
	}
	return nil
}

// ApplyGrants applies grants to a PostgreSQL role on a database
func ApplyGrants(
	ctx context.Context, host string, port int32,
	adminUser, adminPassword, sslMode,
	databaseName, roleName string,
	grants []instancev1alpha1.GrantItem,
	defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem,
) error {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.ApplyGrants",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String(telemetry.AttrDatabase, databaseName),
			attribute.String("db.role", roleName),
			attribute.String(telemetry.AttrOperation, "apply_grants"),
		))
	defer span.End()
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	escapedRole := pq.QuoteIdentifier(roleName)
	escapedDBName := pq.QuoteIdentifier(databaseName)

	var databaseGrants, otherGrants []instancev1alpha1.GrantItem
	for _, grant := range grants {
		if grant.Type == instancev1alpha1.GrantTypeDatabase {
			databaseGrants = append(databaseGrants, grant)
		} else {
			otherGrants = append(otherGrants, grant)
		}
	}

	if len(databaseGrants) > 0 {
		if err := applyDatabaseGrants(
			connCtx, host, port, adminUser, adminPassword,
			sslMode, escapedDBName, escapedRole, databaseGrants,
		); err != nil {
			return err
		}
	}

	if len(otherGrants) > 0 || len(defaultPrivileges) > 0 {
		if err := applyNonDatabaseGrants(
			connCtx, host, port, adminUser, adminPassword,
			sslMode, databaseName, escapedRole,
			otherGrants, defaultPrivileges,
		); err != nil {
			return err
		}
	}

	return nil
}

// escapeGrantIdentifiers escapes the identifiers in a grant item
type grantIdentifiers struct {
	Schema   string
	Table    string
	Sequence string
	Function string
}

// escapeGrantFields escapes the grant item fields
func escapeGrantFields(
	grant instancev1alpha1.GrantItem,
) grantIdentifiers {
	ids := grantIdentifiers{}
	if grant.Schema != "" {
		ids.Schema = pq.QuoteIdentifier(grant.Schema)
	}
	if grant.Table != "" {
		ids.Table = pq.QuoteIdentifier(grant.Table)
	}
	if grant.Sequence != "" {
		ids.Sequence = pq.QuoteIdentifier(grant.Sequence)
	}
	if grant.Function != "" {
		ids.Function = pq.QuoteIdentifier(grant.Function)
	}
	return ids
}

// buildGrantQuery builds the SQL query for a grant item
func buildGrantQuery(
	grant instancev1alpha1.GrantItem,
	privilegesStr, escapedRole string,
	ids grantIdentifiers,
) (string, error) {
	switch grant.Type {
	case instancev1alpha1.GrantTypeSchema:
		if grant.Schema == "" {
			return "", fmt.Errorf("schema is required for schema grant type")
		}
		return fmt.Sprintf("GRANT %s ON SCHEMA %s TO %s",
			privilegesStr, ids.Schema, escapedRole), nil

	case instancev1alpha1.GrantTypeTable:
		if err := requireFields(grant.Schema, "schema", grant.Table, "table", "table"); err != nil {
			return "", err
		}
		return fmt.Sprintf("GRANT %s ON TABLE %s.%s TO %s",
			privilegesStr, ids.Schema, ids.Table, escapedRole), nil

	case instancev1alpha1.GrantTypeSequence:
		if err := requireFields(grant.Schema, "schema", grant.Sequence, "sequence", "sequence"); err != nil {
			return "", err
		}
		return fmt.Sprintf("GRANT %s ON SEQUENCE %s.%s TO %s",
			privilegesStr, ids.Schema, ids.Sequence, escapedRole), nil

	case instancev1alpha1.GrantTypeFunction:
		if err := requireFields(grant.Schema, "schema", grant.Function, "function", "function"); err != nil {
			return "", err
		}
		return fmt.Sprintf("GRANT %s ON FUNCTION %s.%s TO %s",
			privilegesStr, ids.Schema, ids.Function, escapedRole), nil

	case instancev1alpha1.GrantTypeAllTables:
		if grant.Schema == "" {
			return "", fmt.Errorf("schema is required for all_tables grant type")
		}
		return fmt.Sprintf(
			"GRANT %s ON ALL TABLES IN SCHEMA %s TO %s",
			privilegesStr, ids.Schema, escapedRole), nil

	case instancev1alpha1.GrantTypeAllSequences:
		if grant.Schema == "" {
			return "", fmt.Errorf("schema is required for all_sequences grant type")
		}
		return fmt.Sprintf(
			"GRANT %s ON ALL SEQUENCES IN SCHEMA %s TO %s",
			privilegesStr, ids.Schema, escapedRole), nil

	default:
		return "", fmt.Errorf("unknown grant type: %s", grant.Type)
	}
}

// requireFields validates that required fields are non-empty
func requireFields(
	schemaVal, schemaName, fieldVal, fieldName, grantType string,
) error {
	if schemaVal == "" {
		return fmt.Errorf(
			"%s is required for %s grant type", schemaName, grantType)
	}
	if fieldVal == "" {
		return fmt.Errorf(
			"%s is required for %s grant type", fieldName, grantType)
	}
	return nil
}

// applyGrantItem applies a single grant item
func applyGrantItem(
	ctx context.Context, db *sql.DB,
	escapedRole string, grant instancev1alpha1.GrantItem,
) error {
	ids := escapeGrantFields(grant)
	privilegesStr := strings.Join(grant.Privileges, ", ")

	query, err := buildGrantQuery(
		grant, privilegesStr, escapedRole, ids,
	)
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to execute grant query: %w", err)
	}

	return nil
}

// applyDefaultPrivilege applies default privileges for future objects
func applyDefaultPrivilege(
	ctx context.Context, db *sql.DB, escapedRole string, defaultPriv instancev1alpha1.DefaultPrivilegeItem) error {
	escapedSchema := pq.QuoteIdentifier(defaultPriv.Schema)
	privilegesStr := strings.Join(defaultPriv.Privileges, ", ")

	var query string
	switch defaultPriv.ObjectType {
	case "tables":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON TABLES TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON TABLES TO %s",
			escapedSchema, privilegesStr, escapedRole)
	case "sequences":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON SEQUENCES TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON SEQUENCES TO %s",
			escapedSchema, privilegesStr, escapedRole)
	case "functions":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON FUNCTIONS TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON FUNCTIONS TO %s",
			escapedSchema, privilegesStr, escapedRole)
	case "types":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON TYPES TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON TYPES TO %s",
			escapedSchema, privilegesStr, escapedRole)
	default:
		return fmt.Errorf("unknown default privilege object type: %s", defaultPriv.ObjectType)
	}

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to execute default privilege query: %w", err)
	}

	return nil
}
