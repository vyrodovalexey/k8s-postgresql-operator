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

package metrics

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// Collector collects metrics from Kubernetes resources
type Collector struct {
	client.Client
	Log *zap.SugaredLogger
	mu  sync.RWMutex
}

// NewCollector creates a new metrics collector
func NewCollector(c client.Client, log *zap.SugaredLogger) *Collector {
	return &Collector{
		Client: c,
		Log:    log,
	}
}

// CollectMetrics collects metrics from all CRDs
func (c *Collector) CollectMetrics(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Collect PostgreSQL instances
	if err := c.collectPostgresqls(ctx); err != nil {
		return err
	}

	// Collect Users
	if err := c.collectUsers(ctx); err != nil {
		return err
	}

	// Collect Databases
	if err := c.collectDatabases(ctx); err != nil {
		return err
	}

	// Collect Grants
	if err := c.collectGrants(ctx); err != nil {
		return err
	}

	// Collect RoleGroups
	if err := c.collectRoleGroups(ctx); err != nil {
		return err
	}

	// Collect Schemas
	if err := c.collectSchemas(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Collector) collectPostgresqls(ctx context.Context) error {
	var postgresqlList instancev1alpha1.PostgresqlList
	if err := c.List(ctx, &postgresqlList, client.InNamespace("")); err != nil {
		return err
	}

	// Update total count
	UpdatePostgresqlCount(float64(len(postgresqlList.Items)))

	// Track current objects
	currentObjects := make(map[string]bool)
	postgresqlIDCounts := make(map[string]int)

	// Count per postgresqlID (should be 1 per ID, but we'll track it)
	for _, pg := range postgresqlList.Items {
		if pg.Spec.ExternalInstance == nil || pg.Spec.ExternalInstance.PostgresqlID == "" {
			continue
		}
		postgresqlID := pg.Spec.ExternalInstance.PostgresqlID
		postgresqlIDCounts[postgresqlID]++
		key := "postgresql:" + postgresqlID + ":" + pg.Name + ":" + pg.Namespace
		currentObjects[key] = true
		SetObjectInfo("postgresql", postgresqlID, pg.Name, pg.Namespace)
	}

	// Update counts per postgresqlID
	for postgresqlID, count := range postgresqlIDCounts {
		UpdateObjectCount("postgresql", postgresqlID, float64(count))
	}

	// Note: We don't clean up old metrics here as it's complex to track what was deleted.
	// The periodic collection will overwrite with current state, which is sufficient.

	return nil
}

func (c *Collector) collectUsers(ctx context.Context) error {
	var userList instancev1alpha1.UserList
	if err := c.List(ctx, &userList, client.InNamespace("")); err != nil {
		return err
	}

	// Count per postgresqlID
	postgresqlIDCounts := make(map[string]int)
	for _, user := range userList.Items {
		postgresqlID := user.Spec.PostgresqlID
		postgresqlIDCounts[postgresqlID]++
		SetObjectInfo("user", postgresqlID, user.Name, user.Namespace)
	}

	// Update counts per postgresqlID
	for postgresqlID, count := range postgresqlIDCounts {
		UpdateObjectCount("user", postgresqlID, float64(count))
	}

	return nil
}

func (c *Collector) collectDatabases(ctx context.Context) error {
	var databaseList instancev1alpha1.DatabaseList
	if err := c.List(ctx, &databaseList, client.InNamespace("")); err != nil {
		return err
	}

	// Count per postgresqlID
	postgresqlIDCounts := make(map[string]int)
	for _, db := range databaseList.Items {
		postgresqlID := db.Spec.PostgresqlID
		postgresqlIDCounts[postgresqlID]++
		SetObjectInfo("database", postgresqlID, db.Name, db.Namespace)
	}

	// Update counts per postgresqlID
	for postgresqlID, count := range postgresqlIDCounts {
		UpdateObjectCount("database", postgresqlID, float64(count))
	}

	return nil
}

func (c *Collector) collectGrants(ctx context.Context) error {
	var grantList instancev1alpha1.GrantList
	if err := c.List(ctx, &grantList, client.InNamespace("")); err != nil {
		return err
	}

	// Count per postgresqlID
	postgresqlIDCounts := make(map[string]int)
	for _, grant := range grantList.Items {
		postgresqlID := grant.Spec.PostgresqlID
		postgresqlIDCounts[postgresqlID]++
		SetObjectInfo("grant", postgresqlID, grant.Name, grant.Namespace)
	}

	// Update counts per postgresqlID
	for postgresqlID, count := range postgresqlIDCounts {
		UpdateObjectCount("grant", postgresqlID, float64(count))
	}

	return nil
}

func (c *Collector) collectRoleGroups(ctx context.Context) error {
	var roleGroupList instancev1alpha1.RoleGroupList
	if err := c.List(ctx, &roleGroupList, client.InNamespace("")); err != nil {
		return err
	}

	// Count per postgresqlID
	postgresqlIDCounts := make(map[string]int)
	for _, rg := range roleGroupList.Items {
		postgresqlID := rg.Spec.PostgresqlID
		postgresqlIDCounts[postgresqlID]++
		SetObjectInfo("rolegroup", postgresqlID, rg.Name, rg.Namespace)
	}

	// Update counts per postgresqlID
	for postgresqlID, count := range postgresqlIDCounts {
		UpdateObjectCount("rolegroup", postgresqlID, float64(count))
	}

	return nil
}

func (c *Collector) collectSchemas(ctx context.Context) error {
	var schemaList instancev1alpha1.SchemaList
	if err := c.List(ctx, &schemaList, client.InNamespace("")); err != nil {
		return err
	}

	// Count per postgresqlID
	postgresqlIDCounts := make(map[string]int)
	for _, schema := range schemaList.Items {
		postgresqlID := schema.Spec.PostgresqlID
		postgresqlIDCounts[postgresqlID]++
		SetObjectInfo("schema", postgresqlID, schema.Name, schema.Namespace)
	}

	// Update counts per postgresqlID
	for postgresqlID, count := range postgresqlIDCounts {
		UpdateObjectCount("schema", postgresqlID, float64(count))
	}

	return nil
}
