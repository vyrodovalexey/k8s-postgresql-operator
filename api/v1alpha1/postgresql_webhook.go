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

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var postgresqllog = logf.Log.WithName("postgresql-resource")

var (
	webhookClient client.Client
)

// SetWebhookClient sets the client to be used by the webhook validators
func SetWebhookClient(c client.Client) {
	webhookClient = c
}

// SetupWebhookWithManager sets up the webhook with the Manager.
func (r *Postgresql) SetupWebhookWithManager(mgr ctrl.Manager) error {
	SetWebhookClient(mgr.GetClient())
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate,mutating=false,failurePolicy=fail,sideEffects=None,groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=create;update,versions=v1alpha1,name=postgresql-operator.vyrodovalexey.github.com,admissionReviewVersions=v1

var _ admission.CustomValidator = &Postgresql{}

// ValidateCreate implements admission.CustomValidator so a webhook will be registered for the type
func (r *Postgresql) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	postgresql, ok := obj.(*Postgresql)
	if !ok {
		return nil, fmt.Errorf("expected a Postgresql but got a %T", obj)
	}

	postgresqllog.Info("validate create", "name", postgresql.Name)

	if webhookClient == nil {
		return nil, fmt.Errorf("webhook client not initialized")
	}

	// Validate that no other PostgreSQL instance with the same PostgresqlID exists in this namespace
	if postgresql.Spec.ExternalInstance != nil && postgresql.Spec.ExternalInstance.PostgresqlID != "" {
		postgresqlList := &PostgresqlList{}
		if err := webhookClient.List(ctx, postgresqlList, client.InNamespace(postgresql.Namespace)); err != nil {
			return nil, fmt.Errorf("failed to list PostgreSQL instances: %w", err)
		}

		for _, existingPostgresql := range postgresqlList.Items {
			if existingPostgresql.Spec.ExternalInstance != nil &&
				existingPostgresql.Spec.ExternalInstance.PostgresqlID == postgresql.Spec.ExternalInstance.PostgresqlID {
				return nil, fmt.Errorf("PostgreSQL instance with PostgresqlID %s already exists in namespace %s (instance: %s)",
					postgresql.Spec.ExternalInstance.PostgresqlID, postgresql.Namespace, existingPostgresql.Name)
			}
		}
	}

	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator so a webhook will be registered for the type
func (r *Postgresql) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	oldPostgresql, ok := oldObj.(*Postgresql)
	if !ok {
		return nil, fmt.Errorf("expected old object to be a Postgresql but got a %T", oldObj)
	}

	newPostgresql, ok := newObj.(*Postgresql)
	if !ok {
		return nil, fmt.Errorf("expected new object to be a Postgresql but got a %T", newObj)
	}

	postgresqllog.Info("validate update", "name", newPostgresql.Name)

	if webhookClient == nil {
		return nil, fmt.Errorf("webhook client not initialized")
	}

	// If PostgresqlID changed, check for conflicts
	if newPostgresql.Spec.ExternalInstance != nil && oldPostgresql.Spec.ExternalInstance != nil {
		if newPostgresql.Spec.ExternalInstance.PostgresqlID != oldPostgresql.Spec.ExternalInstance.PostgresqlID {
			// PostgresqlID changed, validate it doesn't conflict with existing instances
			postgresqlList := &PostgresqlList{}
			if err := webhookClient.List(ctx, postgresqlList, client.InNamespace(newPostgresql.Namespace)); err != nil {
				return nil, fmt.Errorf("failed to list PostgreSQL instances: %w", err)
			}

			for _, existingPostgresql := range postgresqlList.Items {
				// Skip the current instance
				if existingPostgresql.Name == newPostgresql.Name {
					continue
				}

				if existingPostgresql.Spec.ExternalInstance != nil &&
					existingPostgresql.Spec.ExternalInstance.PostgresqlID == newPostgresql.Spec.ExternalInstance.PostgresqlID {
					return nil, fmt.Errorf("PostgreSQL instance with PostgresqlID %s already exists in namespace %s (instance: %s)",
						newPostgresql.Spec.ExternalInstance.PostgresqlID, newPostgresql.Namespace, existingPostgresql.Name)
				}
			}
		}
	}

	return nil, nil
}

// ValidateDelete implements admission.CustomValidator so a webhook will be registered for the type
func (r *Postgresql) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	postgresql, ok := obj.(*Postgresql)
	if !ok {
		return nil, fmt.Errorf("expected a Postgresql but got a %T", obj)
	}

	postgresqllog.Info("validate delete", "name", postgresql.Name)

	// Allow deletion
	return nil, nil
}
