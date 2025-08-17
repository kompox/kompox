package kompose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubernetes/kompose/pkg/kobject"
	"github.com/kubernetes/kompose/pkg/loader"
	k8stransformer "github.com/kubernetes/kompose/pkg/transformer/kubernetes"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/yaegashi/kompoxops/adapters/kube"
)

// Adapter implements kube.Converter using Kompose low-level APIs.
type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) ComposeToObjects(ctx context.Context, composeYAML []byte, opt kube.ConvertOption) ([]runtime.Object, []string, error) {
	tmpDir, err := os.MkdirTemp("", "kompoxops-kompose-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "compose.yaml")
	if err := os.WriteFile(composePath, composeYAML, 0o600); err != nil {
		return nil, nil, fmt.Errorf("write compose file: %w", err)
	}

	ld, err := loader.GetLoader("compose")
	if err != nil {
		return nil, nil, fmt.Errorf("get loader: %w", err)
	}
	profiles := append([]string{}, opt.Profiles...)
	kObjects, err := ld.LoadFile([]string{composePath}, profiles)
	if err != nil {
		return nil, nil, fmt.Errorf("load file: %w", err)
	}

	replicas := opt.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	controller := opt.Controller
	if controller == "" {
		controller = "deployment"
	}

	convOpt := kobject.ConvertOptions{
		Provider:              "kubernetes",
		Controller:            controller,
		Replicas:              replicas,
		YAMLIndent:            2,
		Profiles:              profiles,
		WithKomposeAnnotation: opt.WithAnnotations,
	}
	transformer := &k8stransformer.Kubernetes{}
	rObjects, err := transformer.Transform(kObjects, convOpt)
	if err != nil {
		return nil, nil, fmt.Errorf("transform: %w", err)
	}
	return rObjects, nil, nil
}
