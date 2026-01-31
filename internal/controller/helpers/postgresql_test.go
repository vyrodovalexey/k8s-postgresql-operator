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

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/constants"
)

func TestGetConnectionDefaults(t *testing.T) {
	tests := []struct {
		name            string
		inputPort       int32
		inputSSLMode    string
		expectedPort    int32
		expectedSSLMode string
	}{
		{
			name:            "Both values provided",
			inputPort:       5433,
			inputSSLMode:    "verify-full",
			expectedPort:    5433,
			expectedSSLMode: "verify-full",
		},
		{
			name:            "Port is zero - use default",
			inputPort:       0,
			inputSSLMode:    "verify-ca",
			expectedPort:    constants.DefaultPostgresPort,
			expectedSSLMode: "verify-ca",
		},
		{
			name:            "SSLMode is empty - use default",
			inputPort:       5432,
			inputSSLMode:    "",
			expectedPort:    5432,
			expectedSSLMode: constants.DefaultSSLMode,
		},
		{
			name:            "Both values are defaults",
			inputPort:       0,
			inputSSLMode:    "",
			expectedPort:    constants.DefaultPostgresPort,
			expectedSSLMode: constants.DefaultSSLMode,
		},
		{
			name:            "Custom port with disable SSL",
			inputPort:       15432,
			inputSSLMode:    "disable",
			expectedPort:    15432,
			expectedSSLMode: "disable",
		},
		{
			name:            "Standard port with require SSL",
			inputPort:       5432,
			inputSSLMode:    "require",
			expectedPort:    5432,
			expectedSSLMode: "require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			port, sslMode := GetConnectionDefaults(tt.inputPort, tt.inputSSLMode)

			// Assert
			assert.Equal(t, tt.expectedPort, port, "Port should match expected value")
			assert.Equal(t, tt.expectedSSLMode, sslMode, "SSLMode should match expected value")
		})
	}
}

func TestGetConnectionDefaults_DefaultValues(t *testing.T) {
	// Verify that the default values are what we expect
	port, sslMode := GetConnectionDefaults(0, "")

	assert.Equal(t, int32(5432), port, "Default port should be 5432")
	assert.Equal(t, "require", sslMode, "Default SSL mode should be 'require'")
}
