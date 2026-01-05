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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IgnoreStatusUpdatesPredicate returns a predicate that ignores status-only updates
// It triggers reconciliation on:
// - Generation changes
// - Deletion timestamp changes
// - Create/Delete/Generic events
func IgnoreStatusUpdatesPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldGen := e.ObjectOld.GetGeneration()
			newGen := e.ObjectNew.GetGeneration()

			if oldGen != newGen {
				return true
			}

			oldDel := e.ObjectOld.GetDeletionTimestamp()
			newDel := e.ObjectNew.GetDeletionTimestamp()

			return (oldDel == nil) != (newDel == nil)
		},
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return true },
	}
}
