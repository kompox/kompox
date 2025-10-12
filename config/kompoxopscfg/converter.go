package kompoxopscfg

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kompox/kompox/domain/model"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ToModels converts the configuration to domain models with proper references.
// Returns models in the order: workspace, provider, cluster, app
func (r *Root) ToModels() (*model.Workspace, *model.Provider, *model.Cluster, *model.App, error) {
	if err := r.Validate(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("invalid configuration: %w", err)
	}

	now := time.Now()

	// Generate UUIDs for each resource
	serviceID, err := generateID()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to generate workspace ID: %w", err)
	}

	providerID, err := generateID()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to generate provider ID: %w", err)
	}

	clusterID, err := generateID()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to generate cluster ID: %w", err)
	}

	appID, err := generateID()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to generate app ID: %w", err)
	}

	// Create Workspace
	workspace := &model.Workspace{
		ID:        serviceID,
		Name:      r.Workspace.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create Provider (references Workspace)
	provider := &model.Provider{
		ID:          providerID,
		Name:        r.Provider.Name,
		WorkspaceID: serviceID,
		Driver:      r.Provider.Driver,
		Settings:    r.Provider.Settings,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create Cluster (references Provider)
	cluster := &model.Cluster{
		ID:         clusterID,
		Name:       r.Cluster.Name,
		ProviderID: providerID,
		Existing:   r.Cluster.Existing,
		Ingress:    toModelClusterIngress(r.Cluster.Ingress),
		Settings:   r.Cluster.Settings,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Handle compose content based on prefix
	composeContent, err := processCompose(r.App.Compose)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to process compose: %w", err)
	}

	// Create App (references Cluster)
	app := &model.App{
		ID:         appID,
		Name:       r.App.Name,
		ClusterID:  clusterID,
		Compose:    composeContent,
		Ingress:    toModelAppIngress(r.App.Ingress),
		Volumes:    toModelVolumes(r.App.Volumes),
		Deployment: toModelAppDeployment(r.App.Deployment),
		Resources:  r.App.Resources,
		Settings:   r.App.Settings,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	return workspace, provider, cluster, app, nil
}

// generateID generates a simple random ID for demo purposes
func generateID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}

// processCompose handles compose content based on the type and prefix
func processCompose(compose any) (string, error) {
	if compose == nil {
		return "", fmt.Errorf("compose value is nil")
	}

	// Check if compose is a string
	if str, ok := compose.(string); ok {
		if strings.HasPrefix(str, "file:") {
			// Extract file path by removing "file:" prefix
			filePath := strings.TrimPrefix(str, "file:")
			return readComposeFile(filePath)
		}
		// Return the string as-is for non-file: prefixes
		return str, nil
	}

	// For non-string types, marshal to YAML
	yamlBytes, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// readComposeFile reads the compose file and returns its content
func readComposeFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("compose file path is empty")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}

	return string(content), nil
}

// toModelIngress converts config slice to domain slice.
func toModelAppIngress(ai AppIngress) model.AppIngress {
	out := model.AppIngress{CertResolver: ai.CertResolver}
	if len(ai.Rules) == 0 {
		return out
	}
	rules := make([]model.AppIngressRule, 0, len(ai.Rules))
	for _, r := range ai.Rules {
		rules = append(rules, model.AppIngressRule{Name: r.Name, Port: r.Port, Hosts: append([]string{}, r.Hosts...)})
	}
	out.Rules = rules
	return out
}

// toModelVolumes converts config volumes to domain volumes.
func toModelVolumes(vs []AppVolume) []model.AppVolume {
	if len(vs) == 0 {
		return nil
	}
	out := make([]model.AppVolume, 0, len(vs))
	for _, v := range vs {
		q, err := resource.ParseQuantity(v.Size)
		if err != nil {
			panic(fmt.Errorf("invalid volume size %q for volume %q: %w", v.Size, v.Name, err))
		}
		// Quantity.Value() returns the value in base units (bytes for memory/storage quantities)
		out = append(out, model.AppVolume{
			Name:    v.Name,
			Size:    q.Value(),
			Options: v.Options,
		})
	}
	return out
}

// toModelAppDeployment converts config AppDeployment to domain AppDeployment.
func toModelAppDeployment(ad AppDeployment) model.AppDeployment {
	return model.AppDeployment{
		Pool: ad.Pool,
		Zone: ad.Zone,
	}
}

// toModelClusterIngress converts config ClusterIngress to domain ClusterIngress pointer.
func toModelClusterIngress(ci ClusterIngress) *model.ClusterIngress {
	// If all fields are empty, return nil to indicate unspecified
	if ci.Namespace == "" && ci.Controller == "" && ci.ServiceAccount == "" && ci.Domain == "" && ci.CertResolver == "" && ci.CertEmail == "" && len(ci.Certificates) == 0 {
		return nil
	}
	// Apply minimal defaults if necessary (ServiceAccount may be empty; runtime has a default)
	mi := &model.ClusterIngress{
		Namespace:      ci.Namespace,
		Controller:     ci.Controller,
		ServiceAccount: ci.ServiceAccount,
		Domain:         ci.Domain,
		CertResolver:   ci.CertResolver,
		CertEmail:      ci.CertEmail,
	}
	if len(ci.Certificates) > 0 {
		certs := make([]model.ClusterIngressCertificate, 0, len(ci.Certificates))
		for _, c := range ci.Certificates {
			certs = append(certs, model.ClusterIngressCertificate{Name: c.Name, Source: c.Source})
		}
		mi.Certificates = certs
	}
	return mi
}
