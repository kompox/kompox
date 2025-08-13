package cfgops

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ToModels converts the configuration to domain models with proper references.
// Returns models in the order: service, provider, cluster, app
func (r *Root) ToModels() (*model.Service, *model.Provider, *model.Cluster, *model.App, error) {
	now := time.Now()

	// Generate UUIDs for each resource
	serviceID, err := generateID()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to generate service ID: %w", err)
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

	// Create Service
	service := &model.Service{
		ID:        serviceID,
		Name:      r.Service.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create Provider (references Service)
	provider := &model.Provider{
		ID:        providerID,
		Name:      r.Provider.Name,
		ServiceID: serviceID,
		Driver:    r.Provider.Driver,
		Settings:  r.Provider.Settings,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create Cluster (references Provider)
	cluster := &model.Cluster{
		ID:         clusterID,
		Name:       r.Cluster.Name,
		ProviderID: providerID,
		Existing:   r.Cluster.Existing,
		Domain:     r.Cluster.Domain,
		Ingress:    r.Cluster.Ingress,
		Settings:   r.Cluster.Settings,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Create App (references Cluster)
	app := &model.App{
		ID:        appID,
		Name:      r.App.Name,
		ClusterID: clusterID,
		Compose:   r.App.Compose,
		Ingress:   r.App.Ingress,
		Resources: r.App.Resources,
		Settings:  r.App.Settings,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return service, provider, cluster, app, nil
}

// generateID generates a simple random ID for demo purposes
func generateID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}
