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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestIgnoreStatusUpdatesPredicate(t *testing.T) {
	predicate := IgnoreStatusUpdatesPredicate()

	t.Run("Create event", func(t *testing.T) {
		obj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		e := event.CreateEvent{
			Object: obj,
		}
		assert.True(t, predicate.Create(e))
	})

	t.Run("Delete event", func(t *testing.T) {
		obj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		e := event.DeleteEvent{
			Object: obj,
		}
		assert.True(t, predicate.Delete(e))
	})

	t.Run("Generic event", func(t *testing.T) {
		obj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		e := event.GenericEvent{
			Object: obj,
		}
		assert.True(t, predicate.Generic(e))
	})

	t.Run("Update event - generation change", func(t *testing.T) {
		oldObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test",
				Generation: 1,
			},
		}
		newObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test",
				Generation: 2,
			},
		}
		e := event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		}
		assert.True(t, predicate.Update(e))
	})

	t.Run("Update event - deletion timestamp change", func(t *testing.T) {
		oldObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				DeletionTimestamp: nil,
			},
		}
		now := metav1.NewTime(time.Now())
		newObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				DeletionTimestamp: &now,
			},
		}
		e := event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		}
		assert.True(t, predicate.Update(e))
	})

	t.Run("Update event - deletion timestamp removed", func(t *testing.T) {
		now := metav1.NewTime(time.Now())
		oldObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				DeletionTimestamp: &now,
			},
		}
		newObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				DeletionTimestamp: nil,
			},
		}
		e := event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		}
		assert.True(t, predicate.Update(e))
	})

	t.Run("Update event - status only update", func(t *testing.T) {
		oldObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test",
				Generation: 1,
			},
		}
		newObj := &instancev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test",
				Generation: 1,
			},
		}
		e := event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		}
		assert.False(t, predicate.Update(e))
	})
}
