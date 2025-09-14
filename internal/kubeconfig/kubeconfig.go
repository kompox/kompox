package kubeconfig

import (
	"fmt"
	"io"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

// Change summarizes a merge result.
type Change struct {
	Action  string
	Count   int
	Current bool
}

// LoadAndNormalize loads kubeconfig bytes and returns a minimal config
// containing a single context, cluster, and authinfo. Optionally renames the
// context to ctxName when non-empty, and sets its default namespace.
func LoadAndNormalize(data []byte, ctxName, nsName string) (*clientcmdapi.Config, error) {
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	// Determine current context name
	curCtxName := cfg.CurrentContext
	if curCtxName == "" {
		if len(cfg.Contexts) != 1 {
			return nil, fmt.Errorf("kubeconfig has no current context")
		}
		for k := range cfg.Contexts {
			curCtxName = k
		}
		cfg.CurrentContext = curCtxName
	} else if cfg.Contexts[curCtxName] == nil {
		return nil, fmt.Errorf("context %q not found in kubeconfig", curCtxName)
	}

	// Keep only the selected context and inline referenced files
	if err := clientcmdapi.MinifyConfig(cfg); err != nil {
		return nil, fmt.Errorf("minify kubeconfig: %w", err)
	}
	if err := clientcmdapi.FlattenConfig(cfg); err != nil {
		return nil, fmt.Errorf("flatten kubeconfig: %w", err)
	}

	curCtxName = cfg.CurrentContext
	curCtx := cfg.Contexts[curCtxName]
	curClusterName := curCtx.Cluster
	curUserName := curCtx.AuthInfo

	// Validate referenced entries exist after minify/flatten
	if _, ok := cfg.Clusters[curClusterName]; !ok {
		return nil, fmt.Errorf("referenced cluster %q not found", curClusterName)
	}
	if _, ok := cfg.AuthInfos[curUserName]; !ok {
		return nil, fmt.Errorf("referenced user %q not found", curUserName)
	}

	// Set namespace if provided
	if nsName != "" {
		curCtx.Namespace = nsName
	}

	// Rename context when a non-empty ctxName differs from the current name
	if ctxName != "" && ctxName != curCtxName {
		cfg.Contexts[ctxName] = curCtx
		delete(cfg.Contexts, curCtxName)
		curCtxName = ctxName
		cfg.CurrentContext = curCtxName
	}

	// Rename cluster to match the context name when ctxName is provided
	if ctxName != "" && ctxName != curClusterName {
		cfg.Clusters[ctxName] = cfg.Clusters[curClusterName]
		delete(cfg.Clusters, curClusterName)
		curCtx.Cluster = ctxName
	}

	// Rename user to match the context name when ctxName is provided
	if ctxName != "" && ctxName != curUserName {
		cfg.AuthInfos[ctxName] = cfg.AuthInfos[curUserName]
		delete(cfg.AuthInfos, curUserName)
		curCtx.AuthInfo = ctxName
	}

	return cfg, nil
}

// MergeIntoExisting merges newCfg into an existing kubeconfig file path.
// It resolves name conflicts; if force=false, it will suffix -1, -2... to obtain unique names.
// Returns merged config, final context name, a change summary, and error.
func MergeIntoExisting(newCfg *clientcmdapi.Config, path string, force, setCurrent bool) (*clientcmdapi.Config, string, Change, error) {
	// Basic validation of input config
	if newCfg == nil || newCfg.CurrentContext == "" {
		return nil, "", Change{}, fmt.Errorf("input kubeconfig has no current context")
	}

	// Determine names of new context, cluster, and user
	newCtxName := newCfg.CurrentContext
	newCtx := newCfg.Contexts[newCtxName]
	if newCtx == nil {
		return nil, "", Change{}, fmt.Errorf("current context %q not found in kubeconfig", newCfg.CurrentContext)
	}
	newClusterName := newCtx.Cluster
	newUserName := newCtx.AuthInfo

	// Validate referenced entries exist
	if _, ok := newCfg.Clusters[newClusterName]; !ok {
		return nil, "", Change{}, fmt.Errorf("referenced cluster %q not found", newClusterName)
	}
	if _, ok := newCfg.AuthInfos[newUserName]; !ok {
		return nil, "", Change{}, fmt.Errorf("referenced user %q not found", newUserName)
	}

	// Load existing; if fails, start from empty
	existingCfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		existingCfg = clientcmdapi.NewConfig()
	}

	if !force {
		// Resolve names to avoid conflicts
		newCtxName = uniqueName(newCtxName, existingCfg.Contexts)
		newClusterName = uniqueName(newClusterName, existingCfg.Clusters)
		newUserName = uniqueName(newUserName, existingCfg.AuthInfos)
	} else {
		// If force and names exist, drop them first
		delete(existingCfg.Contexts, newCtxName)
		delete(existingCfg.Clusters, newClusterName)
		delete(existingCfg.AuthInfos, newUserName)
	}

	existingCfg.Clusters[newClusterName] = newCfg.Clusters[newCtx.Cluster].DeepCopy()
	existingCfg.AuthInfos[newUserName] = newCfg.AuthInfos[newCtx.AuthInfo].DeepCopy()
	existingCfg.Contexts[newCtxName] = newCtx.DeepCopy()
	existingCfg.Contexts[newCtxName].Cluster = newClusterName
	existingCfg.Contexts[newCtxName].AuthInfo = newUserName

	// Summarize changes
	change := Change{Action: "add/update", Count: 3, Current: false}

	// Auto-select the new context when requested or when the existing config has no current context yet
	if setCurrent || existingCfg.CurrentContext == "" {
		existingCfg.CurrentContext = newCtxName
		change.Current = true
	}

	return existingCfg, newCtxName, change, nil
}

// Print prints cfg to writer in yaml or json.
func Print(w io.Writer, cfg *clientcmdapi.Config, format string) error {
	data, err := clientcmd.Write(*cfg)
	if err != nil {
		return fmt.Errorf("serialize kubeconfig: %w", err)
	}
	if format == "json" {
		// convert kubeconfig YAML to JSON
		j, err := yaml.YAMLToJSON(data)
		if err != nil {
			return fmt.Errorf("convert to json: %w", err)
		}
		_, err = w.Write(j)
		return err
	}
	_, err = w.Write(data)
	return err
}

// uniqueName returns name if not present in m; otherwise appends -1, -2... until unique.
func uniqueName[T any](name string, m map[string]T) string {
	if _, ok := m[name]; !ok {
		return name
	}
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d", name, i)
		if _, ok := m[cand]; !ok {
			return cand
		}
	}
}
