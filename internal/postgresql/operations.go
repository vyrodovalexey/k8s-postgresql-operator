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
	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	operrors "github.com/vyrodovalexey/k8s-postgresql-operator/internal/errors"
)

// CreateOrUpdateDatabase creates or updates a PostgreSQL database
func CreateOrUpdateDatabase(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	databaseName, owner, schemaName, templateDatabase string) error {
	// Connect to PostgreSQL as admin user
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, sslMode)

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
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateDatabase.checkDatabaseExists")
	}

	if exists {
		// Update database owner
		query := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", escapedDatabaseName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateDatabase.updateDatabaseOwner")
		}
		return nil
	}

	// Create new database
	var query string
	if templateDatabase != "" {
		// Create database from template
		escapedTemplate := pq.QuoteIdentifier(templateDatabase)
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s TEMPLATE %s", escapedDatabaseName, escapedOwner, escapedTemplate)
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
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateDatabase.checkSchemaExists")
	}

	if !schemaExists {
		return nil
	}

	// Update schema owner
	query = fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
	_, err = dbSchema.ExecContext(connCtx, query)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateDatabase.updateSchemaOwner")
	}

	return nil
}

// CreateOrUpdateUser creates or updates a PostgreSQL user
func CreateOrUpdateUser(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, username, password string) error {
	// Connect to PostgreSQL as admin user
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, sslMode)

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
	escapedPassword := pq.QuoteIdentifier(password)

	// Check if user exists
	var exists bool
	err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateUser.checkUserExists")
	}

	if exists {
		// Update user password - PostgreSQL doesn't support parameters in ALTER USER
		query := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateUser.updateUserPassword")
		}
	} else {
		// Create new user - PostgreSQL doesn't support parameters in CREATE USER
		query := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateUser.createUser")
		}
	}

	return nil
}

// CreateOrUpdateRoleGroup creates or updates a PostgreSQL role group
func CreateOrUpdateRoleGroup(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	groupRole string, memberRoles []string) error {
	// Connect to PostgreSQL as admin user
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlConnectionFailed, err).WithOp("CreateOrUpdateRoleGroup.sql.Open")
	}
	defer db.Close()

	// Escape group role name for SQL identifier
	escapedGroupRole := pq.QuoteIdentifier(groupRole)

	// Check if group role exists
	var exists bool
	err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", groupRole).Scan(&exists)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).
			WithOp("CreateOrUpdateRoleGroup.checkGroupRoleExists")
	}

	if !exists {
		// Create the group role (as a role, not a user, so it can be a group)
		query := fmt.Sprintf("CREATE ROLE %s", escapedGroupRole)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateRoleGroup.createGroupRole")
		}
	}

	// Get current members of the group role
	currentMembers := make(map[string]bool)
	rows, err := db.QueryContext(connCtx, `
		SELECT r.rolname 
		FROM pg_roles r 
		JOIN pg_auth_members m ON r.oid = m.member 
		JOIN pg_roles g ON g.oid = m.roleid 
		WHERE g.rolname = $1`, groupRole)
	if err != nil {
		return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp("CreateOrUpdateRoleGroup.queryCurrentMembers")
	}
	defer rows.Close()

	for rows.Next() {
		var member string
		if err := rows.Scan(&member); err != nil {
			return fmt.Errorf("failed to scan member: %w", err)
		}
		currentMembers[member] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating members: %w", err)
	}

	// Add new members that are not already members
	desiredMembers := make(map[string]bool)
	for _, memberRole := range memberRoles {
		desiredMembers[memberRole] = true
		if !currentMembers[memberRole] {
			// Check if member role exists
			var memberExists bool
			err = db.QueryRowContext(connCtx,
				"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", memberRole).Scan(&memberExists)
			if err != nil {
				return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).
					WithOp("CreateOrUpdateRoleGroup.checkMemberRoleExists")
			}
			if !memberExists {
				// Skip non-existent members
				continue
			}

			// Add member to group
			escapedMemberRole := pq.QuoteIdentifier(memberRole)
			query := fmt.Sprintf("GRANT %s TO %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				op := fmt.Sprintf("CreateOrUpdateRoleGroup.addMemberToGroup(%s)", memberRole)
				return operrors.Wrap(operrors.ErrPostgresqlOperationFailed, err).WithOp(op)
			}
		}
	}

	// Remove members that are no longer in the desired list
	for member := range currentMembers {
		if !desiredMembers[member] {
			escapedMemberRole := pq.QuoteIdentifier(member)
			query := fmt.Sprintf("REVOKE %s FROM %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("failed to remove member %s from group role: %w", member, err)
			}
		}
	}

	return nil
}

// CreateOrUpdateSchema creates or updates a PostgreSQL schema
func CreateOrUpdateSchema(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	databaseName, schemaName, owner string) error {
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

// ApplyGrants applies grants to a PostgreSQL role on a database
func ApplyGrants(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	databaseName, roleName string, grants []instancev1alpha1.GrantItem,
	defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem) error {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Escape role name for SQL identifier
	escapedRole := pq.QuoteIdentifier(roleName)
	escapedDBName := pq.QuoteIdentifier(databaseName)

	// Separate database grants from other grants
	var databaseGrants []instancev1alpha1.GrantItem
	var otherGrants []instancev1alpha1.GrantItem
	for _, grant := range grants {
		if grant.Type == instancev1alpha1.GrantTypeDatabase {
			databaseGrants = append(databaseGrants, grant)
		} else {
			otherGrants = append(otherGrants, grant)
		}
	}

	// Apply database grants - must be done from postgres database
	if len(databaseGrants) > 0 {
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
			host, port, adminUser, adminPassword, sslMode)

		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return fmt.Errorf("failed to open database connection to postgres: %w", err)
		}
		defer db.Close()

		// Test connection
		if err := db.PingContext(connCtx); err != nil {
			return fmt.Errorf("failed to ping postgres database: %w", err)
		}

		// Apply database grants
		for _, grant := range databaseGrants {
			privilegesStr := strings.Join(grant.Privileges, ", ")
			query := fmt.Sprintf("GRANT %s ON DATABASE %s TO %s", privilegesStr, escapedDBName, escapedRole)
			if _, err := db.ExecContext(connCtx, query); err != nil {
				return fmt.Errorf("failed to apply database grant: %w", err)
			}
		}
	}

	// Apply other grants and default privileges - must be done from the target database
	if len(otherGrants) > 0 || len(defaultPrivileges) > 0 {
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
			host, port, adminUser, adminPassword, databaseName, sslMode)

		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return fmt.Errorf("failed to open database connection to %s: %w", databaseName, err)
		}
		defer db.Close()

		// Test connection
		if err := db.PingContext(connCtx); err != nil {
			return fmt.Errorf("failed to ping database %s: %w", databaseName, err)
		}

		// Apply regular grants
		for _, grant := range otherGrants {
			if err := applyGrantItem(connCtx, db, escapedRole, grant); err != nil {
				return fmt.Errorf("failed to apply grant %s: %w", grant.Type, err)
			}
		}

		// Apply default privileges
		for _, defaultPriv := range defaultPrivileges {
			if err := applyDefaultPrivilege(connCtx, db, escapedRole, defaultPriv); err != nil {
				return fmt.Errorf("failed to apply default privilege for %s: %w", defaultPriv.ObjectType, err)
			}
		}
	}

	return nil
}

// applyGrantItem applies a single grant item
func applyGrantItem(ctx context.Context, db *sql.DB, escapedRole string, grant instancev1alpha1.GrantItem) error {
	// Escape schema, table, sequence, function names
	escapedSchema := ""
	if grant.Schema != "" {
		escapedSchema = pq.QuoteIdentifier(grant.Schema)
	}

	escapedTable := ""
	if grant.Table != "" {
		escapedTable = pq.QuoteIdentifier(grant.Table)
	}

	escapedSequence := ""
	if grant.Sequence != "" {
		escapedSequence = pq.QuoteIdentifier(grant.Sequence)
	}

	escapedFunction := ""
	if grant.Function != "" {
		escapedFunction = pq.QuoteIdentifier(grant.Function)
	}

	// Build privileges string
	privilegesStr := strings.Join(grant.Privileges, ", ")

	var query string
	switch grant.Type {
	case instancev1alpha1.GrantTypeSchema:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for schema grant type")
		}
		// GRANT privileges ON SCHEMA schema_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON SCHEMA %s TO %s", privilegesStr, escapedSchema, escapedRole)

	case instancev1alpha1.GrantTypeTable:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for table grant type")
		}
		if grant.Table == "" {
			return fmt.Errorf("table is required for table grant type")
		}
		// GRANT privileges ON TABLE schema_name.table_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON TABLE %s.%s TO %s", privilegesStr, escapedSchema, escapedTable, escapedRole)

	case instancev1alpha1.GrantTypeSequence:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for sequence grant type")
		}
		if grant.Sequence == "" {
			return fmt.Errorf("sequence is required for sequence grant type")
		}
		// GRANT privileges ON SEQUENCE schema_name.sequence_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON SEQUENCE %s.%s TO %s", privilegesStr, escapedSchema, escapedSequence, escapedRole)

	case instancev1alpha1.GrantTypeFunction:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for function grant type")
		}
		if grant.Function == "" {
			return fmt.Errorf("function is required for function grant type")
		}
		// GRANT privileges ON FUNCTION schema_name.function_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON FUNCTION %s.%s TO %s", privilegesStr, escapedSchema, escapedFunction, escapedRole)

	case instancev1alpha1.GrantTypeAllTables:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for all_tables grant type")
		}
		// GRANT privileges ON ALL TABLES IN SCHEMA schema_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON ALL TABLES IN SCHEMA %s TO %s", privilegesStr, escapedSchema, escapedRole)

	case instancev1alpha1.GrantTypeAllSequences:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for all_sequences grant type")
		}
		// GRANT privileges ON ALL SEQUENCES IN SCHEMA schema_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON ALL SEQUENCES IN SCHEMA %s TO %s", privilegesStr, escapedSchema, escapedRole)

	default:
		return fmt.Errorf("unknown grant type: %s", grant.Type)
	}

	_, err := db.ExecContext(ctx, query)
	if err != nil {
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
