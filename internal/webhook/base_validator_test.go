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
	"fmt"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	appconfig "github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
)

// MockManager implements manager.Manager for testing
type MockManager struct {
	mock.Mock
	mockClient client.Client
}

func (m *MockManager) Add(runnable manager.Runnable) error {
	args := m.Called(runnable)
	return args.Error(0)
}

func (m *MockManager) Elected() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *MockManager) AddMetricsServerExtraHandler(_ string, _ http.Handler) error {
	return nil
}

func (m *MockManager) AddHealthzCheck(_ string, _ healthz.Checker) error {
	return nil
}

func (m *MockManager) AddReadyzCheck(_ string, _ healthz.Checker) error {
	return nil
}

func (m *MockManager) Start(_ context.Context) error {
	return nil
}

func (m *MockManager) GetConfig() *rest.Config {
	return &rest.Config{}
}

func (m *MockManager) GetScheme() *runtime.Scheme {
	return runtime.NewScheme()
}

func (m *MockManager) GetClient() client.Client {
	return m.mockClient
}

func (m *MockManager) GetFieldIndexer() client.FieldIndexer {
	return nil
}

func (m *MockManager) GetCache() cache.Cache {
	return nil
}

func (m *MockManager) GetEventRecorderFor(_ string) record.EventRecorder {
	return nil
}

func (m *MockManager) GetRESTMapper() meta.RESTMapper {
	return nil
}

func (m *MockManager) GetAPIReader() client.Reader {
	return nil
}

func (m *MockManager) GetWebhookServer() webhook.Server {
	return nil
}

func (m *MockManager) GetLogger() logr.Logger {
	return logr.Discard()
}

func (m *MockManager) GetControllerOptions() ctrlconfig.Controller {
	return ctrlconfig.Controller{}
}

func (m *MockManager) GetHTTPClient() *http.Client {
	return nil
}

// MockWebhookServer implements webhook.Server for testing
type MockWebhookServer struct {
	mock.Mock
	registeredPaths []string
}

func (m *MockWebhookServer) NeedLeaderElection() bool {
	return false
}

func (m *MockWebhookServer) Register(path string, _ http.Handler) {
	m.registeredPaths = append(m.registeredPaths, path)
}

func (m *MockWebhookServer) Start(_ context.Context) error {
	return nil
}

func (m *MockWebhookServer) StartedChecker() healthz.Checker {
	return nil
}

func (m *MockWebhookServer) WebhookMux() *http.ServeMux {
	return http.NewServeMux()
}

func TestNewBaseValidatorConfig(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	mgr := &MockManager{mockClient: mockClient}
	cfg := appconfig.New()

	baseCfg := NewBaseValidatorConfig(mgr, mockDecoder, nil, logger, cfg)

	assert.Equal(t, mockClient, baseCfg.Client)
	assert.Equal(t, mockDecoder, baseCfg.Decoder)
	assert.Equal(t, logger, baseCfg.Log)
	assert.Nil(t, baseCfg.VaultClient)
	assert.Equal(t, cfg.PostgresqlConnectionRetries, baseCfg.PostgresqlConnectionRetries)
	assert.Equal(t, cfg.PostgresqlConnectionTimeout(), baseCfg.PostgresqlConnectionTimeout)
	assert.Equal(t, cfg.VaultAvailabilityRetries, baseCfg.VaultAvailabilityRetries)
	assert.Equal(t, cfg.VaultAvailabilityRetryDelay(), baseCfg.VaultAvailabilityRetryDelay)
}

func TestRegisterWebhooks_Success(t *testing.T) {
	mockClient := new(MockWebhookClient)
	logger := zap.NewNop().Sugar()
	mockDecoder := new(MockDecoder)

	mgr := &MockManager{mockClient: mockClient}
	mgr.On("Add", mock.Anything).Return(nil)

	webhookServer := &MockWebhookServer{}
	cfg := appconfig.New()

	err := RegisterWebhooks(mgr, webhookServer, mockDecoder, cfg, nil, []string{}, logger)
	assert.NoError(t, err)

	// Verify all 6 webhook paths were registered
	expectedPaths := []string{
		"/uservalidate",
		"/postgresqlvalidate",
		"/databasevalidate",
		"/grantvalidate",
		"/rolegroupvalidate",
		"/schemavalidate",
	}
	assert.Equal(t, expectedPaths, webhookServer.registeredPaths)
	mgr.AssertCalled(t, "Add", mock.Anything)
}

func TestRegisterWebhooks_AddError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	logger := zap.NewNop().Sugar()
	mockDecoder := new(MockDecoder)

	mgr := &MockManager{mockClient: mockClient}
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("failed to add webhook server"))

	webhookServer := &MockWebhookServer{}
	cfg := appconfig.New()

	err := RegisterWebhooks(mgr, webhookServer, mockDecoder, cfg, nil, []string{}, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to set up webhook server")
}
