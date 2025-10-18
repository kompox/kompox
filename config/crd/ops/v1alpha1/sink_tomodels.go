package v1alpha1

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/kompox/kompox/domain/model"
)

// ToModels converts the CRD Sink to domain models and populates the provided repositories.
// This method is analogous to kompoxopscfg.Root.ToModels() but for CRD sources.
// It creates domain models in dependency order: Workspace → Provider → Cluster → App.
//
// The conversion process:
//  1. Iterates through CRD resources in the sink
//  2. Extracts Resource ID from annotations
//  3. Sets parent FKs to parent Resource ID
//  4. Persists models to the provided repositories
//
// kompoxAppFilePath is the absolute path of the Kompox app file (e.g., kompoxapp.yml).
// Apps defined in this file have RefBase set to "file://<dir>/"; apps from external KOM
// sources have RefBase set to "" (disallowing local references).
//
// Returns an error if any resource fails to convert or if Resource ID extraction fails.
func (s *Sink) ToModels(ctx context.Context, repos Repositories, kompoxAppFilePath string) error {
	// Create workspaces
	for _, ws := range s.ListWorkspaces() {
		fqn, err := ExtractResourceID("Workspace", ws.ObjectMeta.Name, ws.ObjectMeta.Annotations)
		if err != nil {
			return fmt.Errorf("failed to extract Resource ID for workspace %q: %w", ws.ObjectMeta.Name, err)
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
		// Extract Resource ID from annotation
		fqn, err := ExtractResourceID("Provider", prv.ObjectMeta.Name, prv.ObjectMeta.Annotations)
		if err != nil {
			return fmt.Errorf("failed to extract Resource ID for provider %q: %w", prv.ObjectMeta.Name, err)
		}

		// Parent workspace ID is the parent FQN
		wsID := fqn.ParentFQN().String()

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
		fqn, err := ExtractResourceID("Cluster", cls.ObjectMeta.Name, cls.ObjectMeta.Annotations)
		if err != nil {
			return fmt.Errorf("failed to extract Resource ID for cluster %q: %w", cls.ObjectMeta.Name, err)
		}

		// Parent provider ID is the parent FQN
		prvID := fqn.ParentFQN().String()

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
		fqn, err := ExtractResourceID("App", app.ObjectMeta.Name, app.ObjectMeta.Annotations)
		if err != nil {
			return fmt.Errorf("failed to extract Resource ID for app %q: %w", app.ObjectMeta.Name, err)
		}

		// Parent cluster ID is the parent FQN
		clsID := fqn.ParentFQN().String()

		// Determine RefBase based on app origin
		// Apps from kompoxapp.yml get file:// RefBase; external KOM apps get empty RefBase
		sourceFile := app.ObjectMeta.Annotations[AnnotationDocPath]
		refBase := ""
		if kompoxAppFilePath != "" && sourceFile == kompoxAppFilePath {
			// App is from the Kompox app file; allow local references
			baseDir := filepath.Dir(sourceFile)
			refBase = "file://" + baseDir + "/"
		}
		// else: external KOM origin, RefBase remains empty (disallow local references)

		domainApp := &model.App{
			ID:        fqn.String(),
			Name:      app.ObjectMeta.Name,
			ClusterID: clsID,
			Compose:   app.Spec.Compose, // Keep as-is (no file: expansion)
			RefBase:   refBase,
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
