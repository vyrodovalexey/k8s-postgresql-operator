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

package webhook

import (
	"context"
	"time"

	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// testPostgreSQLConnection tests the connection to a PostgreSQL instance
func testPostgreSQLConnection(
	ctx context.Context,
	postgresqlObj *instancev1alpha1.Postgresql,
	vaultClient *vault.Client,
	log *zap.SugaredLogger,
	retries int,
	retryTimeout time.Duration) error {
	return pg.TestConnectionFromPostgresql(ctx, postgresqlObj, vaultClient, log, retries, retryTimeout)
}
