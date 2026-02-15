package main

import (
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestKubeconfigContextNamespaceMatches(t *testing.T) {
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")

	matched, err := kubeconfigContextNamespaceMatches(kubeconfigPath, "ctx-a", "ns-a")
	if err != nil {
		t.Fatalf("unexpected error for missing kubeconfig: %v", err)
	}
	if matched {
		t.Fatalf("expected no match for missing kubeconfig")
	}

	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["cluster-a"] = &clientcmdapi.Cluster{Server: "https://example.invalid"}
	cfg.AuthInfos["user-a"] = &clientcmdapi.AuthInfo{Token: "token"}
	cfg.Contexts["ctx-a"] = &clientcmdapi.Context{Cluster: "cluster-a", AuthInfo: "user-a", Namespace: "ns-a"}
	cfg.CurrentContext = "ctx-a"
	if err := clientcmd.WriteToFile(*cfg, kubeconfigPath); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	matched, err = kubeconfigContextNamespaceMatches(kubeconfigPath, "ctx-a", "ns-a")
	if err != nil {
		t.Fatalf("unexpected error for matched kubeconfig: %v", err)
	}
	if !matched {
		t.Fatalf("expected matched context+namespace")
	}

	matched, err = kubeconfigContextNamespaceMatches(kubeconfigPath, "ctx-a", "ns-b")
	if err != nil {
		t.Fatalf("unexpected error for namespace mismatch: %v", err)
	}
	if matched {
		t.Fatalf("expected mismatch when namespace differs")
	}

	matched, err = kubeconfigContextNamespaceMatches(kubeconfigPath, "ctx-b", "ns-a")
	if err != nil {
		t.Fatalf("unexpected error for context mismatch: %v", err)
	}
	if matched {
		t.Fatalf("expected mismatch when context is missing")
	}
}
