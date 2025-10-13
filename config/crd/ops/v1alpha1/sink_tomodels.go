package v1alpha1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kompox/kompox/domain/model"
)

// ToModels converts the CRD Sink to domain models and populates the provided repositories.
// This method is analogous to kompoxopscfg.Root.ToModels() but for CRD sources.
// It creates domain models in dependency order: Workspace → Provider → Cluster → App.
//
// The conversion process:
//  1. Iterates through CRD resources in the sink
//  2. Builds FQN for each resource and sets it as the ID
//  3. Sets parent FKs to parent FQN (no query needed since parent IDs are also FQNs)
//  4. Persists models to the provided repositories
//
// Returns an error if any resource fails to convert or if FQN construction fails.
func (s *Sink) ToModels(ctx context.Context, repos Repositories) error {
	// Create workspaces
	for _, ws := range s.ListWorkspaces() {
		fqn, err := BuildFQN("Workspace", "", ws.ObjectMeta.Name)
		if err != nil {
			return fmt.Errorf("failed to build FQN for workspace %q: %w", ws.ObjectMeta.Name, err)
		}
		workspace := &model.Workspace{
			ID:   fqn.String(),
			Name: ws.ObjectMeta.Name,
		}
		if err := repos.Workspace.Create(ctx, workspace); err != nil {
			return fmt.Errorf("failed to create workspace %q: %w", ws.ObjectMeta.Name, err)
		}
	}

	// Create providers
	for _, prv := range s.ListProviders() {
		// Build FQN from annotation path and name
		parentPath := prv.ObjectMeta.Annotations[AnnotationPath]
		fqn, err := BuildFQN("Provider", parentPath, prv.ObjectMeta.Name)
		if err != nil {
			return fmt.Errorf("failed to build FQN for provider %q: %w", prv.ObjectMeta.Name, err)
		}

		// Parent workspace FQN is the parentPath itself
		wsID := parentPath

		provider := &model.Provider{
			ID:          fqn.String(),
			Name:        prv.ObjectMeta.Name,
			WorkspaceID: wsID,
			Driver:      prv.Spec.Driver,
			Settings:    prv.Spec.Settings,
		}
		if err := repos.Provider.Create(ctx, provider); err != nil {
			return fmt.Errorf("failed to create provider %q: %w", prv.ObjectMeta.Name, err)
		}
	}

	// Create clusters
	for _, cls := range s.ListClusters() {
		parentPath := cls.ObjectMeta.Annotations[AnnotationPath]
		fqn, err := BuildFQN("Cluster", parentPath, cls.ObjectMeta.Name)
		if err != nil {
			return fmt.Errorf("failed to build FQN for cluster %q: %w", cls.ObjectMeta.Name, err)
		}

		// Parent provider FQN is the parentPath itself
		prvID := parentPath

		cluster := &model.Cluster{
			ID:         fqn.String(),
			Name:       cls.ObjectMeta.Name,
			ProviderID: prvID,
			Existing:   cls.Spec.Existing,
			Settings:   cls.Spec.Settings,
		}
		if cls.Spec.Ingress != nil {
			cluster.Ingress = &model.ClusterIngress{
				Namespace:      cls.Spec.Ingress.Namespace,
				Controller:     cls.Spec.Ingress.Controller,
				ServiceAccount: cls.Spec.Ingress.ServiceAccount,
				Domain:         cls.Spec.Ingress.Domain,
				CertResolver:   cls.Spec.Ingress.CertResolver,
				CertEmail:      cls.Spec.Ingress.CertEmail,
			}
			if len(cls.Spec.Ingress.Certificates) > 0 {
				certs := make([]model.ClusterIngressCertificate, 0, len(cls.Spec.Ingress.Certificates))
				for _, c := range cls.Spec.Ingress.Certificates {
					certs = append(certs, model.ClusterIngressCertificate{
						Name:   c.Name,
						Source: c.Source,
					})
				}
				cluster.Ingress.Certificates = certs
			}
		}
		if err := repos.Cluster.Create(ctx, cluster); err != nil {
			return fmt.Errorf("failed to create cluster %q: %w", cls.ObjectMeta.Name, err)
		}
	}

	// Create apps
	for _, app := range s.ListApps() {
		parentPath := app.ObjectMeta.Annotations[AnnotationPath]
		fqn, err := BuildFQN("App", parentPath, app.ObjectMeta.Name)
		if err != nil {
			return fmt.Errorf("failed to build FQN for app %q: %w", app.ObjectMeta.Name, err)
		}

		// Parent cluster FQN is the parentPath itself
		clsID := parentPath

		// Get source file directory for relative path resolution
		sourceFile := app.ObjectMeta.Annotations[AnnotationDocPath]
		baseDir := filepath.Dir(sourceFile)

		// Process compose field (load file content if needed)
		compose, err := processCompose(app.Spec.Compose, baseDir)
		if err != nil {
			return fmt.Errorf("failed to process compose for app %q: %w", app.ObjectMeta.Name, err)
		}

		domainApp := &model.App{
			ID:        fqn.String(),
			Name:      app.ObjectMeta.Name,
			ClusterID: clsID,
			Compose:   compose,
			Resources: app.Spec.Resources,
			Settings:  app.Spec.Settings,
		}

		// Convert Ingress if present
		if app.Spec.Ingress != nil {
			domainApp.Ingress = model.AppIngress{
				CertResolver: app.Spec.Ingress.CertResolver,
			}
			if len(app.Spec.Ingress.Rules) > 0 {
				rules := make([]model.AppIngressRule, 0, len(app.Spec.Ingress.Rules))
				for _, r := range app.Spec.Ingress.Rules {
					rules = append(rules, model.AppIngressRule{
						Name:  r.Name,
						Port:  r.Port,
						Hosts: r.Hosts,
					})
				}
				domainApp.Ingress.Rules = rules
			}
		}

		// Convert Volumes if present
		if len(app.Spec.Volumes) > 0 {
			volumes := make([]model.AppVolume, 0, len(app.Spec.Volumes))
			for _, v := range app.Spec.Volumes {
				volumes = append(volumes, model.AppVolume{
					Name:    v.Name,
					Size:    v.Size,
					Options: v.Options,
				})
			}
			domainApp.Volumes = volumes
		}

		// Convert Deployment if present
		if app.Spec.Deployment != nil {
			domainApp.Deployment = model.AppDeployment{
				Pool: app.Spec.Deployment.Pool,
				Zone: app.Spec.Deployment.Zone,
			}
		}

		if err := repos.App.Create(ctx, domainApp); err != nil {
			return fmt.Errorf("failed to create app %q: %w", app.ObjectMeta.Name, err)
		}
	}

	return nil
}

// Repositories defines the repository interfaces needed for converting CRD to domain models.
type Repositories struct {
	Workspace WorkspaceRepository
	Provider  ProviderRepository
	Cluster   ClusterRepository
	App       AppRepository
}

// WorkspaceRepository defines operations for workspace persistence.
type WorkspaceRepository interface {
	Create(ctx context.Context, ws *model.Workspace) error
	List(ctx context.Context) ([]*model.Workspace, error)
}

// ProviderRepository defines operations for provider persistence.
type ProviderRepository interface {
	Create(ctx context.Context, prv *model.Provider) error
	List(ctx context.Context) ([]*model.Provider, error)
}

// ClusterRepository defines operations for cluster persistence.
type ClusterRepository interface {
	Create(ctx context.Context, cls *model.Cluster) error
	List(ctx context.Context) ([]*model.Cluster, error)
}

// AppRepository defines operations for app persistence.
type AppRepository interface {
	Create(ctx context.Context, app *model.App) error
	List(ctx context.Context) ([]*model.App, error)
}

// processCompose processes the compose field, loading file content if needed.
// If the compose string has a "file:" prefix, it reads the file relative to baseDir.
// Returns the processed compose content (either inline or file content).
func processCompose(compose string, baseDir string) (string, error) {
	if compose == "" {
		return "", nil
	}

	if !strings.HasPrefix(compose, "file:") {
		// Inline compose, return as-is
		return compose, nil
	}

	// Extract file path (remove "file:" prefix)
	filePath := strings.TrimPrefix(compose, "file:")

	// Resolve relative paths based on baseDir
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(baseDir, filePath)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("reading compose file %q: %w", filePath, err)
	}

	return string(content), nil
}
