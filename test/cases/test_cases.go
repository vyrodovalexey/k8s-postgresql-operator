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

// Package cases contains shared test case definitions for the k8s-postgresql-operator.
// These test cases can be used across functional, integration, and e2e tests.
package cases

// PostgreSQLTestCase defines a test case for PostgreSQL operations.
type PostgreSQLTestCase struct {
	Name          string
	DatabaseName  string
	Owner         string
	SchemaName    string
	TemplateName  string
	ExpectError   bool
	ErrorContains string
}

// VaultTestCase defines a test case for Vault operations.
type VaultTestCase struct {
	Name          string
	MountPath     string
	SecretPath    string
	Data          map[string]interface{}
	ExpectError   bool
	ErrorContains string
}

// UserTestCase defines a test case for user operations.
type UserTestCase struct {
	Name          string
	Username      string
	Password      string
	ExpectError   bool
	ErrorContains string
}

// GrantTestCase defines a test case for grant operations.
type GrantTestCase struct {
	Name          string
	RoleName      string
	DatabaseName  string
	SchemaName    string
	Privileges    []string
	GrantType     string
	ExpectError   bool
	ErrorContains string
}

// RoleGroupTestCase defines a test case for role group operations.
type RoleGroupTestCase struct {
	Name          string
	GroupRole     string
	MemberRoles   []string
	ExpectError   bool
	ErrorContains string
}

// ConfigTestCase defines a test case for config validation.
type ConfigTestCase struct {
	Name          string
	EnvKey        string
	EnvValue      string
	ExpectedField string
	ExpectedValue interface{}
}
