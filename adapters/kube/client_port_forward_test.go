package kube_test

import (
	"context"
	"testing"
	"time"

	"github.com/kompox/kompox/adapters/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestPortForwardOptions_Validation(t *testing.T) {
	// Create a client with fake clientset
	client := &kube.Client{
		Clientset: fake.NewSimpleClientset(),
	}

	tests := []struct {
		name    string
		opts    *kube.PortForwardOptions
		wantErr bool
	}{
		{
			name:    "nil options",
			opts:    nil,
			wantErr: true,
		},
		{
			name: "missing pod name",
			opts: &kube.PortForwardOptions{
				Namespace:  "test-ns",
				RemotePort: 8080,
			},
			wantErr: true,
		},
		{
			name: "missing namespace",
			opts: &kube.PortForwardOptions{
				PodName:    "test-pod",
				RemotePort: 8080,
			},
			wantErr: true,
		},
		{
			name: "invalid remote port",
			opts: &kube.PortForwardOptions{
				Namespace:  "test-ns",
				PodName:    "test-pod",
				RemotePort: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			// This will fail during parameter validation before trying to access k8s API
			_, err := client.PortForward(ctx, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("PortForward() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFindPodByLabels_Validation(t *testing.T) {
	// Create a fake client with some test pods
	testPods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ready-pod",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app": "test",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:  "main",
						Ready: true,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-ready-pod",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app": "test",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:  "main",
						Ready: false,
					},
				},
			},
		},
	}

	fakeClientset := fake.NewSimpleClientset()
	for _, pod := range testPods {
		_, err := fakeClientset.CoreV1().Pods("test-ns").Create(context.Background(), &pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create test pod: %v", err)
		}
	}

	client := &kube.Client{
		Clientset: fakeClientset,
	}

	tests := []struct {
		name          string
		namespace     string
		labelSelector string
		wantPodName   string
		wantErr       bool
	}{
		{
			name:          "empty namespace",
			namespace:     "",
			labelSelector: "app=test",
			wantErr:       true,
		},
		{
			name:          "empty label selector",
			namespace:     "test-ns",
			labelSelector: "",
			wantErr:       true,
		},
		{
			name:          "no matching pods",
			namespace:     "test-ns",
			labelSelector: "app=nonexistent",
			wantErr:       true,
		},
		{
			name:          "finds ready pod",
			namespace:     "test-ns",
			labelSelector: "app=test",
			wantPodName:   "ready-pod", // Should prefer ready pod
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pod, err := client.FindPodByLabels(ctx, tt.namespace, tt.labelSelector)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindPodByLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && pod.Name != tt.wantPodName {
				t.Errorf("FindPodByLabels() pod name = %v, want %v", pod.Name, tt.wantPodName)
			}
		})
	}
}

func TestClientFromKubeconfig(t *testing.T) {
	// Create a minimal kubeconfig for testing
	kubeConfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"test-cluster": {
				Server: "https://localhost:6443",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"test-context": {
				Cluster:   "test-cluster",
				AuthInfo:  "test-user",
				Namespace: "default",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test-user": {
				Token: "test-token",
			},
		},
		CurrentContext: "test-context",
	}

	kubeconfigBytes, err := clientcmd.Write(kubeConfig)
	if err != nil {
		t.Fatalf("Failed to marshal kubeconfig: %v", err)
	}

	ctx := context.Background()
	client, err := kube.NewClientFromKubeconfig(ctx, kubeconfigBytes, nil)
	if err != nil {
		t.Fatalf("NewClientFromKubeconfig() error = %v", err)
	}

	if client.RESTConfig == nil {
		t.Error("Expected RESTConfig to be set")
	}
	if client.Clientset == nil {
		t.Error("Expected Clientset to be set")
	}

	// Test that the kubeconfig is preserved
	retrievedKubeconfig := client.Kubeconfig()
	if len(retrievedKubeconfig) == 0 {
		t.Error("Expected kubeconfig bytes to be preserved")
	}
}
