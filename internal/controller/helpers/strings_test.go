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

	"github.com/stretchr/testify/assert"
)

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		s        string
		expected bool
	}{
		{
			name:     "String found",
			slice:    []string{"a", "b", "c"},
			s:        "b",
			expected: true,
		},
		{
			name:     "String not found",
			slice:    []string{"a", "b", "c"},
			s:        "d",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			s:        "a",
			expected: false,
		},
		{
			name:     "Empty string in slice",
			slice:    []string{"a", "", "c"},
			s:        "",
			expected: true,
		},
		{
			name:     "Case sensitive",
			slice:    []string{"a", "b", "c"},
			s:        "A",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsString(tt.slice, tt.s)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		s        string
		expected []string
	}{
		{
			name:     "Remove existing string",
			slice:    []string{"a", "b", "c"},
			s:        "b",
			expected: []string{"a", "c"},
		},
		{
			name:     "Remove non-existing string",
			slice:    []string{"a", "b", "c"},
			s:        "d",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Remove from empty slice",
			slice:    []string{},
			s:        "a",
			expected: nil, // RemoveString returns nil for empty input (no allocations)
		},
		{
			name:     "Remove all occurrences",
			slice:    []string{"a", "b", "a", "c", "a"},
			s:        "a",
			expected: []string{"b", "c"},
		},
		{
			name:     "Remove last element",
			slice:    []string{"a", "b", "c"},
			s:        "c",
			expected: []string{"a", "b"},
		},
		{
			name:     "Remove first element",
			slice:    []string{"a", "b", "c"},
			s:        "a",
			expected: []string{"b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveString(tt.slice, tt.s)
			assert.Equal(t, tt.expected, result)
		})
	}
}
