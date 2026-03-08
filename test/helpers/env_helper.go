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
	"os"
	"testing"
)

// SkipIfEnvNotSet skips the test if any of the specified environment variables are not set.
func SkipIfEnvNotSet(t *testing.T, envVars ...string) {
	t.Helper()
	for _, envVar := range envVars {
		if os.Getenv(envVar) == "" {
			t.Skipf("Skipping test: environment variable %s is not set", envVar)
		}
	}
}

// GetEnvOrDefault returns the value of the environment variable or the default value if not set.
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
