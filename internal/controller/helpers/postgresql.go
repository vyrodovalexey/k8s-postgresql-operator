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
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/constants"
)

// GetConnectionDefaults returns the port and sslMode with default values applied if not specified
// This consolidates the default value logic used across all controllers
func GetConnectionDefaults(port int32, sslMode string) (defaultPort int32, defaultSSLMode string) {
	if port == 0 {
		port = constants.DefaultPostgresPort
	}
	if sslMode == "" {
		sslMode = constants.DefaultSSLMode
	}
	return port, sslMode
}
