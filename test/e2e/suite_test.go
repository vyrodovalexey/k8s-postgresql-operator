//go:build e2e

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

var (
	testConfig  *testutils.TestConfig
	testEnv     *envtest.Environment
	k8sClient   client.Client
	k8sConfig   *rest.Config
	testScheme  *runtime.Scheme
	testTimeout = 60 * time.Second
)

func TestMain(m *testing.M) {
	testConfig = testutils.DefaultTestConfig()

	// Setup scheme
	testScheme = runtime.NewScheme()
	if err := scheme.AddToScheme(testScheme); err != nil {
		fmt.Printf("Failed to add client-go scheme: %v\n", err)
		os.Exit(1)
	}
	if err := instancev1alpha1.AddToScheme(testScheme); err != nil {
		fmt.Printf("Failed to add instancev1alpha1 scheme: %v\n", err)
		os.Exit(1)
	}

	// Check if we should use envtest or real cluster
	useRealCluster := os.Getenv("USE_REAL_CLUSTER") == "true"

	if useRealCluster {
		// Use real Kubernetes cluster
		var err error
		k8sConfig, err = getKubeConfig()
		if err != nil {
			fmt.Printf("Failed to get Kubernetes config: %v\n", err)
			os.Exit(1)
		}

		k8sClient, err = client.New(k8sConfig, client.Options{Scheme: testScheme})
		if err != nil {
			fmt.Printf("Failed to create Kubernetes client: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Use envtest
		testEnv = &envtest.Environment{
			CRDDirectoryPaths:     []string{"../../config/crd/bases"},
			ErrorIfCRDPathMissing: true,
		}

		var err error
		k8sConfig, err = testEnv.Start()
		if err != nil {
			fmt.Printf("Failed to start envtest: %v\n", err)
			os.Exit(1)
		}

		k8sClient, err = client.New(k8sConfig, client.Options{Scheme: testScheme})
		if err != nil {
			fmt.Printf("Failed to create Kubernetes client: %v\n", err)
			os.Exit(1)
		}
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if testEnv != nil {
		if err := testEnv.Stop(); err != nil {
			fmt.Printf("Failed to stop envtest: %v\n", err)
		}
	}

	os.Exit(code)
}

// getKubeConfig returns the Kubernetes config from kubeconfig file
func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// createTestNamespace creates a test namespace
func createTestNamespace(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := k8sClient.Create(ctx, ns); err != nil {
		if !errors.IsAlreadyExists(err) {
			t.Fatalf("Failed to create namespace: %v", err)
		}
	}
}

// deleteTestNamespace deletes a test namespace
func deleteTestNamespace(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := k8sClient.Delete(ctx, ns); err != nil {
		if !errors.IsNotFound(err) {
			t.Logf("Warning: Failed to delete namespace: %v", err)
		}
	}
}

// waitForCondition waits for a condition to be met
func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("Timeout waiting for condition: %s", message)
}
