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

package helpers

import (
	"context"
	"fmt"
	"time"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PostgresqlInstanceInfo contains information about a PostgreSQL instance
type PostgresqlInstanceInfo struct {
	Postgresql       *instancev1alpha1.Postgresql
	ExternalInstance *instancev1alpha1.ExternalPostgresqlInstance
	Port             int32
	SSLMode          string
}

// FindAndValidatePostgresql finds a PostgreSQL instance by ID and validates it
// Returns the PostgreSQL instance info or an error if not found or invalid
func FindAndValidatePostgresql(
	ctx context.Context, k8sClient client.Client, postgresqlID string) (*PostgresqlInstanceInfo, error) {
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, k8sClient, postgresqlID)
	if err != nil {
		return nil, fmt.Errorf("postgresql instance with ID %s not found: %w", postgresqlID, err)
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		return nil, fmt.Errorf("postgresql instance with ID %s is not connected", postgresqlID)
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		return nil, fmt.Errorf("postgresql instance has no external instance configuration")
	}

	port := externalInstance.Port
	if port == 0 {
		port = pg.DefaultPort
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = pg.DefaultSSLMode
	}

	return &PostgresqlInstanceInfo{
		Postgresql:       postgresql,
		ExternalInstance: externalInstance,
		Port:             port,
		SSLMode:          sslMode,
	}, nil
}

// RequeueAfter returns a reconcile result that requeues after the specified duration
func RequeueAfter(duration time.Duration) (interface{}, error) {
	return struct {
		RequeueAfter time.Duration
	}{RequeueAfter: duration}, nil
}
