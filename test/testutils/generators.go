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

package testutils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// GenerateTestName generates a unique test name with a random suffix
func GenerateTestName(prefix string) string {
	suffix := generateRandomString(8)
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

// GenerateTestNamespace generates a unique test namespace
func GenerateTestNamespace() string {
	return GenerateTestName("test-ns")
}

// GenerateTestUsername generates a unique test username
func GenerateTestUsername() string {
	return GenerateTestName("testuser")
}

// GenerateTestDatabaseName generates a unique test database name
func GenerateTestDatabaseName() string {
	return GenerateTestName("testdb")
}

// GenerateTestSchemaName generates a unique test schema name
func GenerateTestSchemaName() string {
	return GenerateTestName("testschema")
}

// GenerateTestRoleName generates a unique test role name
func GenerateTestRoleName() string {
	return GenerateTestName("testrole")
}

// GenerateTestPassword generates a random password
func GenerateTestPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return generateRandomStringFromCharset(length, charset)
}

// generateRandomString generates a random lowercase alphanumeric string
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	return generateRandomStringFromCharset(length, charset)
}

// generateRandomStringFromCharset generates a random string from a given charset
func generateRandomStringFromCharset(length int, charset string) string {
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to timestamp-based if crypto/rand fails
			result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		} else {
			result[i] = charset[num.Int64()]
		}
	}
	return string(result)
}

// PostgresqlBuilder helps build Postgresql CRD objects for testing
type PostgresqlBuilder struct {
	postgresql *instancev1alpha1.Postgresql
}

// NewPostgresqlBuilder creates a new PostgresqlBuilder
func NewPostgresqlBuilder() *PostgresqlBuilder {
	return &PostgresqlBuilder{
		postgresql: &instancev1alpha1.Postgresql{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateTestName("postgresql"),
				Namespace: "default",
			},
			Spec: instancev1alpha1.PostgresqlSpec{},
		},
	}
}

// WithName sets the name
func (b *PostgresqlBuilder) WithName(name string) *PostgresqlBuilder {
	b.postgresql.Name = name
	return b
}

// WithNamespace sets the namespace
func (b *PostgresqlBuilder) WithNamespace(namespace string) *PostgresqlBuilder {
	b.postgresql.Namespace = namespace
	return b
}

// WithExternalInstance sets the external instance configuration
func (b *PostgresqlBuilder) WithExternalInstance(postgresqlID, address string, port int32, sslMode string) *PostgresqlBuilder {
	b.postgresql.Spec.ExternalInstance = &instancev1alpha1.ExternalPostgresqlInstance{
		PostgresqlID: postgresqlID,
		Address:      address,
		Port:         port,
		SSLMode:      sslMode,
	}
	return b
}

// WithConnectedStatus sets the connected status
func (b *PostgresqlBuilder) WithConnectedStatus(connected bool) *PostgresqlBuilder {
	b.postgresql.Status.Connected = connected
	return b
}

// Build returns the built Postgresql object
func (b *PostgresqlBuilder) Build() *instancev1alpha1.Postgresql {
	return b.postgresql
}

// UserBuilder helps build User CRD objects for testing
type UserBuilder struct {
	user *instancev1alpha1.User
}

// NewUserBuilder creates a new UserBuilder
func NewUserBuilder() *UserBuilder {
	return &UserBuilder{
		user: &instancev1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateTestName("user"),
				Namespace: "default",
			},
			Spec: instancev1alpha1.UserSpec{},
		},
	}
}

// WithName sets the name
func (b *UserBuilder) WithName(name string) *UserBuilder {
	b.user.Name = name
	return b
}

// WithNamespace sets the namespace
func (b *UserBuilder) WithNamespace(namespace string) *UserBuilder {
	b.user.Namespace = namespace
	return b
}

// WithUsername sets the PostgreSQL username
func (b *UserBuilder) WithUsername(username string) *UserBuilder {
	b.user.Spec.Username = username
	return b
}

// WithPostgresqlID sets the PostgreSQL ID
func (b *UserBuilder) WithPostgresqlID(postgresqlID string) *UserBuilder {
	b.user.Spec.PostgresqlID = postgresqlID
	return b
}

// WithUpdatePassword sets the updatePassword flag
func (b *UserBuilder) WithUpdatePassword(updatePassword bool) *UserBuilder {
	b.user.Spec.UpdatePassword = updatePassword
	return b
}

// Build returns the built User object
func (b *UserBuilder) Build() *instancev1alpha1.User {
	return b.user
}

// DatabaseBuilder helps build Database CRD objects for testing
type DatabaseBuilder struct {
	database *instancev1alpha1.Database
}

// NewDatabaseBuilder creates a new DatabaseBuilder
func NewDatabaseBuilder() *DatabaseBuilder {
	return &DatabaseBuilder{
		database: &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateTestName("database"),
				Namespace: "default",
			},
			Spec: instancev1alpha1.DatabaseSpec{},
		},
	}
}

// WithName sets the name
func (b *DatabaseBuilder) WithName(name string) *DatabaseBuilder {
	b.database.Name = name
	return b
}

// WithNamespace sets the namespace
func (b *DatabaseBuilder) WithNamespace(namespace string) *DatabaseBuilder {
	b.database.Namespace = namespace
	return b
}

// WithDatabaseName sets the PostgreSQL database name
func (b *DatabaseBuilder) WithDatabaseName(dbName string) *DatabaseBuilder {
	b.database.Spec.Database = dbName
	return b
}

// WithOwner sets the database owner
func (b *DatabaseBuilder) WithOwner(owner string) *DatabaseBuilder {
	b.database.Spec.Owner = owner
	return b
}

// WithSchema sets the schema name
func (b *DatabaseBuilder) WithSchema(schema string) *DatabaseBuilder {
	b.database.Spec.Schema = schema
	return b
}

// WithPostgresqlID sets the PostgreSQL ID
func (b *DatabaseBuilder) WithPostgresqlID(postgresqlID string) *DatabaseBuilder {
	b.database.Spec.PostgresqlID = postgresqlID
	return b
}

// WithDeleteFromCRD sets the deleteFromCRD flag
func (b *DatabaseBuilder) WithDeleteFromCRD(deleteFromCRD bool) *DatabaseBuilder {
	b.database.Spec.DeleteFromCRD = deleteFromCRD
	return b
}

// WithDBTemplate sets the database template
func (b *DatabaseBuilder) WithDBTemplate(template string) *DatabaseBuilder {
	b.database.Spec.DBTemplate = template
	return b
}

// Build returns the built Database object
func (b *DatabaseBuilder) Build() *instancev1alpha1.Database {
	return b.database
}

// GrantBuilder helps build Grant CRD objects for testing
type GrantBuilder struct {
	grant *instancev1alpha1.Grant
}

// NewGrantBuilder creates a new GrantBuilder
func NewGrantBuilder() *GrantBuilder {
	return &GrantBuilder{
		grant: &instancev1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateTestName("grant"),
				Namespace: "default",
			},
			Spec: instancev1alpha1.GrantSpec{},
		},
	}
}

// WithName sets the name
func (b *GrantBuilder) WithName(name string) *GrantBuilder {
	b.grant.Name = name
	return b
}

// WithNamespace sets the namespace
func (b *GrantBuilder) WithNamespace(namespace string) *GrantBuilder {
	b.grant.Namespace = namespace
	return b
}

// WithRole sets the role name
func (b *GrantBuilder) WithRole(role string) *GrantBuilder {
	b.grant.Spec.Role = role
	return b
}

// WithDatabase sets the database name
func (b *GrantBuilder) WithDatabase(database string) *GrantBuilder {
	b.grant.Spec.Database = database
	return b
}

// WithPostgresqlID sets the PostgreSQL ID
func (b *GrantBuilder) WithPostgresqlID(postgresqlID string) *GrantBuilder {
	b.grant.Spec.PostgresqlID = postgresqlID
	return b
}

// WithGrants sets the grants
func (b *GrantBuilder) WithGrants(grants []instancev1alpha1.GrantItem) *GrantBuilder {
	b.grant.Spec.Grants = grants
	return b
}

// AddDatabaseGrant adds a database grant
func (b *GrantBuilder) AddDatabaseGrant(privileges ...string) *GrantBuilder {
	b.grant.Spec.Grants = append(b.grant.Spec.Grants, instancev1alpha1.GrantItem{
		Type:       instancev1alpha1.GrantTypeDatabase,
		Privileges: privileges,
	})
	return b
}

// AddSchemaGrant adds a schema grant
func (b *GrantBuilder) AddSchemaGrant(schema string, privileges ...string) *GrantBuilder {
	b.grant.Spec.Grants = append(b.grant.Spec.Grants, instancev1alpha1.GrantItem{
		Type:       instancev1alpha1.GrantTypeSchema,
		Schema:     schema,
		Privileges: privileges,
	})
	return b
}

// AddTableGrant adds a table grant
func (b *GrantBuilder) AddTableGrant(schema, table string, privileges ...string) *GrantBuilder {
	b.grant.Spec.Grants = append(b.grant.Spec.Grants, instancev1alpha1.GrantItem{
		Type:       instancev1alpha1.GrantTypeTable,
		Schema:     schema,
		Table:      table,
		Privileges: privileges,
	})
	return b
}

// AddAllTablesGrant adds an all tables grant
func (b *GrantBuilder) AddAllTablesGrant(schema string, privileges ...string) *GrantBuilder {
	b.grant.Spec.Grants = append(b.grant.Spec.Grants, instancev1alpha1.GrantItem{
		Type:       instancev1alpha1.GrantTypeAllTables,
		Schema:     schema,
		Privileges: privileges,
	})
	return b
}

// WithDefaultPrivileges sets the default privileges
func (b *GrantBuilder) WithDefaultPrivileges(defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem) *GrantBuilder {
	b.grant.Spec.DefaultPrivileges = defaultPrivileges
	return b
}

// Build returns the built Grant object
func (b *GrantBuilder) Build() *instancev1alpha1.Grant {
	return b.grant
}

// RoleGroupBuilder helps build RoleGroup CRD objects for testing
type RoleGroupBuilder struct {
	roleGroup *instancev1alpha1.RoleGroup
}

// NewRoleGroupBuilder creates a new RoleGroupBuilder
func NewRoleGroupBuilder() *RoleGroupBuilder {
	return &RoleGroupBuilder{
		roleGroup: &instancev1alpha1.RoleGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateTestName("rolegroup"),
				Namespace: "default",
			},
			Spec: instancev1alpha1.RoleGroupSpec{},
		},
	}
}

// WithName sets the name
func (b *RoleGroupBuilder) WithName(name string) *RoleGroupBuilder {
	b.roleGroup.Name = name
	return b
}

// WithNamespace sets the namespace
func (b *RoleGroupBuilder) WithNamespace(namespace string) *RoleGroupBuilder {
	b.roleGroup.Namespace = namespace
	return b
}

// WithGroupRole sets the group role name
func (b *RoleGroupBuilder) WithGroupRole(groupRole string) *RoleGroupBuilder {
	b.roleGroup.Spec.GroupRole = groupRole
	return b
}

// WithMemberRoles sets the member roles
func (b *RoleGroupBuilder) WithMemberRoles(memberRoles ...string) *RoleGroupBuilder {
	b.roleGroup.Spec.MemberRoles = memberRoles
	return b
}

// WithPostgresqlID sets the PostgreSQL ID
func (b *RoleGroupBuilder) WithPostgresqlID(postgresqlID string) *RoleGroupBuilder {
	b.roleGroup.Spec.PostgresqlID = postgresqlID
	return b
}

// Build returns the built RoleGroup object
func (b *RoleGroupBuilder) Build() *instancev1alpha1.RoleGroup {
	return b.roleGroup
}

// SchemaBuilder helps build Schema CRD objects for testing
type SchemaBuilder struct {
	schema *instancev1alpha1.Schema
}

// NewSchemaBuilder creates a new SchemaBuilder
func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{
		schema: &instancev1alpha1.Schema{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateTestName("schema"),
				Namespace: "default",
			},
			Spec: instancev1alpha1.SchemaSpec{},
		},
	}
}

// WithName sets the name
func (b *SchemaBuilder) WithName(name string) *SchemaBuilder {
	b.schema.Name = name
	return b
}

// WithNamespace sets the namespace
func (b *SchemaBuilder) WithNamespace(namespace string) *SchemaBuilder {
	b.schema.Namespace = namespace
	return b
}

// WithSchemaName sets the PostgreSQL schema name
func (b *SchemaBuilder) WithSchemaName(schemaName string) *SchemaBuilder {
	b.schema.Spec.Schema = schemaName
	return b
}

// WithOwner sets the schema owner
func (b *SchemaBuilder) WithOwner(owner string) *SchemaBuilder {
	b.schema.Spec.Owner = owner
	return b
}

// WithPostgresqlID sets the PostgreSQL ID
func (b *SchemaBuilder) WithPostgresqlID(postgresqlID string) *SchemaBuilder {
	b.schema.Spec.PostgresqlID = postgresqlID
	return b
}

// Build returns the built Schema object
func (b *SchemaBuilder) Build() *instancev1alpha1.Schema {
	return b.schema
}

// TestCase represents a test case for table-driven tests
type TestCase struct {
	Name        string
	Description string
	Setup       func() error
	Cleanup     func() error
	Validate    func() error
	ExpectError bool
	ErrorMsg    string
}

// SanitizeName sanitizes a name for use in PostgreSQL (lowercase, no special chars)
func SanitizeName(name string) string {
	// Convert to lowercase and replace invalid characters
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name
}
