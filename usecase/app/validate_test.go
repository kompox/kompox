package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/domain/model"
)

const (
	testAppID       = "/ws/ws1/prv/prv1/cls/cls1/app/app1"
	testClusterID   = "/ws/ws1/prv/prv1/cls/cls1"
	testProviderID  = "/ws/ws1/prv/prv1"
	testWorkspaceID = "/ws/ws1"
)

var composeYAML = `services:
  app:
    image: nginx
    volumes:
      - data:/data
volumes:
  data: {}
`

func init() {
	providerdrv.Register("fake-app-validate", func(_ *model.Workspace, _ *model.Provider) (providerdrv.Driver, error) {
		return &fakeProviderDriver{volumeClass: model.VolumeClass{CSIDriver: "fake.csi", StorageClassName: "fake-sc"}}, nil
	})
}

func TestValidateWarnsOnMissingVolumeAssignments(t *testing.T) {
	uc := buildTestUseCase(t, map[string][]*model.VolumeDisk{})
	out, err := uc.Validate(context.Background(), &ValidateInput{AppID: testAppID})
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", out.Errors)
	}
	if len(out.Warnings) == 0 {
		t.Fatalf("expected warning for missing assignment")
	}
	if got := out.Warnings[0]; !strings.Contains(got, "volume assignment missing") {
		t.Fatalf("unexpected warning message: %s", got)
	}
	if out.Compose == "" {
		t.Fatalf("expected compose output to be populated")
	}
}

func TestValidateErrorsOnMultipleAssignments(t *testing.T) {
	uc := buildTestUseCase(t, map[string][]*model.VolumeDisk{
		"data": {
			{Name: "d1", VolumeName: "data", Assigned: true, Handle: "handle-1"},
			{Name: "d2", VolumeName: "data", Assigned: true, Handle: "handle-2"},
		},
	})
	out, err := uc.Validate(context.Background(), &ValidateInput{AppID: testAppID})
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected 1 error, got %v", out.Errors)
	}
	if got := out.Errors[0]; !strings.Contains(got, "count=2") {
		t.Fatalf("unexpected error message: %s", got)
	}
}

func buildTestUseCase(t *testing.T, disks map[string][]*model.VolumeDisk) *UseCase {
	t.Helper()
	app := &model.App{
		ID:        testAppID,
		Name:      "app1",
		ClusterID: testClusterID,
		Compose:   composeYAML,
		Volumes: []model.AppVolume{
			{Name: "data", Size: 1 << 30},
		},
	}
	cluster := &model.Cluster{ID: testClusterID, ProviderID: testProviderID, Name: "cls1"}
	provider := &model.Provider{ID: testProviderID, WorkspaceID: testWorkspaceID, Driver: "fake-app-validate"}
	workspace := &model.Workspace{ID: testWorkspaceID, Name: "ws1"}
	return &UseCase{
		Repos: &Repos{
			App:       &singleAppRepo{item: app},
			Box:       &emptyBoxRepo{},
			Workspace: &singleWorkspaceRepo{item: workspace},
			Provider:  &singleProviderRepo{item: provider},
			Cluster:   &singleClusterRepo{item: cluster},
		},
		VolumePort: &fakeVolumePort{disks: disks},
	}
}

type singleAppRepo struct{ item *model.App }

func (r *singleAppRepo) Create(context.Context, *model.App) error {
	return errors.New("not implemented")
}
func (r *singleAppRepo) Get(_ context.Context, id string) (*model.App, error) {
	if r.item != nil && (id == "" || id == r.item.ID) {
		return r.item, nil
	}
	return nil, nil
}
func (r *singleAppRepo) List(context.Context) ([]*model.App, error) { return []*model.App{r.item}, nil }
func (r *singleAppRepo) Update(context.Context, *model.App) error {
	return errors.New("not implemented")
}
func (r *singleAppRepo) Delete(context.Context, string) error { return errors.New("not implemented") }

type emptyBoxRepo struct{}

func (r *emptyBoxRepo) Create(context.Context, *model.Box) error {
	return errors.New("not implemented")
}
func (r *emptyBoxRepo) Get(context.Context, string) (*model.Box, error) {
	return nil, model.ErrBoxNotFound
}
func (r *emptyBoxRepo) List(context.Context) ([]*model.Box, error) {
	return []*model.Box{}, nil
}
func (r *emptyBoxRepo) ListByAppID(context.Context, string) ([]*model.Box, error) {
	return []*model.Box{}, nil
}
func (r *emptyBoxRepo) Update(context.Context, *model.Box) error {
	return errors.New("not implemented")
}
func (r *emptyBoxRepo) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

type singleClusterRepo struct{ item *model.Cluster }

func (r *singleClusterRepo) Create(context.Context, *model.Cluster) error {
	return errors.New("not implemented")
}
func (r *singleClusterRepo) Get(_ context.Context, id string) (*model.Cluster, error) {
	if r.item != nil && (id == "" || id == r.item.ID) {
		return r.item, nil
	}
	return nil, nil
}
func (r *singleClusterRepo) List(context.Context) ([]*model.Cluster, error) {
	return []*model.Cluster{r.item}, nil
}
func (r *singleClusterRepo) Update(context.Context, *model.Cluster) error {
	return errors.New("not implemented")
}
func (r *singleClusterRepo) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

type singleProviderRepo struct{ item *model.Provider }

func (r *singleProviderRepo) Create(context.Context, *model.Provider) error {
	return errors.New("not implemented")
}
func (r *singleProviderRepo) Get(_ context.Context, id string) (*model.Provider, error) {
	if r.item != nil && (id == "" || id == r.item.ID) {
		return r.item, nil
	}
	return nil, nil
}
func (r *singleProviderRepo) List(context.Context) ([]*model.Provider, error) {
	return []*model.Provider{r.item}, nil
}
func (r *singleProviderRepo) Update(context.Context, *model.Provider) error {
	return errors.New("not implemented")
}
func (r *singleProviderRepo) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

type singleWorkspaceRepo struct{ item *model.Workspace }

func (r *singleWorkspaceRepo) Create(context.Context, *model.Workspace) error {
	return errors.New("not implemented")
}
func (r *singleWorkspaceRepo) Get(_ context.Context, id string) (*model.Workspace, error) {
	if r.item != nil && (id == "" || id == r.item.ID) {
		return r.item, nil
	}
	return nil, nil
}
func (r *singleWorkspaceRepo) List(context.Context) ([]*model.Workspace, error) {
	return []*model.Workspace{r.item}, nil
}
func (r *singleWorkspaceRepo) Update(context.Context, *model.Workspace) error {
	return errors.New("not implemented")
}
func (r *singleWorkspaceRepo) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

type fakeVolumePort struct {
	disks map[string][]*model.VolumeDisk
}

func (f *fakeVolumePort) DiskList(_ context.Context, _ *model.Cluster, _ *model.App, volName string, _ ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	return f.disks[volName], nil
}
func (f *fakeVolumePort) DiskCreate(context.Context, *model.Cluster, *model.App, string, string, string, ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeVolumePort) DiskDelete(context.Context, *model.Cluster, *model.App, string, string, ...model.VolumeDiskDeleteOption) error {
	return errors.New("not implemented")
}
func (f *fakeVolumePort) DiskAssign(context.Context, *model.Cluster, *model.App, string, string, ...model.VolumeDiskAssignOption) error {
	return errors.New("not implemented")
}
func (f *fakeVolumePort) SnapshotList(context.Context, *model.Cluster, *model.App, string, ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeVolumePort) SnapshotCreate(context.Context, *model.Cluster, *model.App, string, string, string, ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeVolumePort) SnapshotDelete(context.Context, *model.Cluster, *model.App, string, string, ...model.VolumeSnapshotDeleteOption) error {
	return errors.New("not implemented")
}

type fakeProviderDriver struct {
	volumeClass model.VolumeClass
}

func (f *fakeProviderDriver) ID() string            { return "fake-app-validate" }
func (f *fakeProviderDriver) WorkspaceName() string { return "ws1" }
func (f *fakeProviderDriver) ProviderName() string  { return "prv1" }
func (f *fakeProviderDriver) ClusterProvision(context.Context, *model.Cluster, ...model.ClusterProvisionOption) error {
	return nil
}
func (f *fakeProviderDriver) ClusterDeprovision(context.Context, *model.Cluster, ...model.ClusterDeprovisionOption) error {
	return nil
}
func (f *fakeProviderDriver) ClusterStatus(context.Context, *model.Cluster) (*model.ClusterStatus, error) {
	return nil, nil
}
func (f *fakeProviderDriver) ClusterInstall(context.Context, *model.Cluster, ...model.ClusterInstallOption) error {
	return nil
}
func (f *fakeProviderDriver) ClusterUninstall(context.Context, *model.Cluster, ...model.ClusterUninstallOption) error {
	return nil
}
func (f *fakeProviderDriver) ClusterKubeconfig(context.Context, *model.Cluster) ([]byte, error) {
	return []byte("kubeconfig"), nil
}
func (f *fakeProviderDriver) ClusterDNSApply(context.Context, *model.Cluster, model.DNSRecordSet, ...model.ClusterDNSApplyOption) error {
	return nil
}
func (f *fakeProviderDriver) VolumeDiskList(context.Context, *model.Cluster, *model.App, string, ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	return nil, nil
}
func (f *fakeProviderDriver) VolumeDiskCreate(context.Context, *model.Cluster, *model.App, string, string, string, ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	return nil, nil
}
func (f *fakeProviderDriver) VolumeDiskDelete(context.Context, *model.Cluster, *model.App, string, string, ...model.VolumeDiskDeleteOption) error {
	return nil
}
func (f *fakeProviderDriver) VolumeDiskAssign(context.Context, *model.Cluster, *model.App, string, string, ...model.VolumeDiskAssignOption) error {
	return nil
}
func (f *fakeProviderDriver) VolumeSnapshotList(context.Context, *model.Cluster, *model.App, string, ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	return nil, nil
}
func (f *fakeProviderDriver) VolumeSnapshotCreate(context.Context, *model.Cluster, *model.App, string, string, string, ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	return nil, nil
}
func (f *fakeProviderDriver) VolumeSnapshotDelete(context.Context, *model.Cluster, *model.App, string, string, ...model.VolumeSnapshotDeleteOption) error {
	return nil
}
func (f *fakeProviderDriver) VolumeClass(context.Context, *model.Cluster, *model.App, model.AppVolume) (model.VolumeClass, error) {
	return f.volumeClass, nil
}
func (f *fakeProviderDriver) NodePoolList(context.Context, *model.Cluster, ...model.NodePoolListOption) ([]*model.NodePool, error) {
	return nil, nil
}
func (f *fakeProviderDriver) NodePoolCreate(context.Context, *model.Cluster, model.NodePool, ...model.NodePoolCreateOption) (*model.NodePool, error) {
	return nil, nil
}
func (f *fakeProviderDriver) NodePoolUpdate(context.Context, *model.Cluster, model.NodePool, ...model.NodePoolUpdateOption) (*model.NodePool, error) {
	return nil, nil
}
func (f *fakeProviderDriver) NodePoolDelete(context.Context, *model.Cluster, string, ...model.NodePoolDeleteOption) error {
	return nil
}
