package v1alpha1

import (
	"testing"
)

func TestFQN_Segments(t *testing.T) {
	tests := []struct {
		name string
		fqn  FQN
		want []string
	}{
		{
			name: "workspace",
			fqn:  "ws1",
			want: []string{"ws1"},
		},
		{
			name: "provider",
			fqn:  "ws1/prv1",
			want: []string{"ws1", "prv1"},
		},
		{
			name: "cluster",
			fqn:  "ws1/prv1/cls1",
			want: []string{"ws1", "prv1", "cls1"},
		},
		{
			name: "app",
			fqn:  "ws1/prv1/cls1/app1",
			want: []string{"ws1", "prv1", "cls1", "app1"},
		},
		{
			name: "box",
			fqn:  "ws1/prv1/cls1/app1/box1",
			want: []string{"ws1", "prv1", "cls1", "app1", "box1"},
		},
		{
			name: "empty",
			fqn:  "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fqn.Segments()
			if len(got) != len(tt.want) {
				t.Errorf("Segments() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Segments()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFQN_ParentFQN(t *testing.T) {
	tests := []struct {
		name string
		fqn  FQN
		want FQN
	}{
		{
			name: "workspace has no parent",
			fqn:  "ws1",
			want: "",
		},
		{
			name: "provider parent is workspace",
			fqn:  "ws1/prv1",
			want: "ws1",
		},
		{
			name: "cluster parent is provider",
			fqn:  "ws1/prv1/cls1",
			want: "ws1/prv1",
		},
		{
			name: "app parent is cluster",
			fqn:  "ws1/prv1/cls1/app1",
			want: "ws1/prv1/cls1",
		},
		{
			name: "box parent is app",
			fqn:  "ws1/prv1/cls1/app1/box1",
			want: "ws1/prv1/cls1/app1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fqn.ParentFQN(); got != tt.want {
				t.Errorf("ParentFQN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateSegmentCount(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		path    string
		wantErr bool
	}{
		{
			name:    "workspace valid",
			kind:    "Workspace",
			path:    "ws1",
			wantErr: false,
		},
		{
			name:    "workspace too many segments",
			kind:    "Workspace",
			path:    "ws1/extra",
			wantErr: true,
		},
		{
			name:    "provider valid",
			kind:    "Provider",
			path:    "ws1/prv1",
			wantErr: false,
		},
		{
			name:    "provider too few segments",
			kind:    "Provider",
			path:    "ws1",
			wantErr: true,
		},
		{
			name:    "cluster valid",
			kind:    "Cluster",
			path:    "ws1/prv1/cls1",
			wantErr: false,
		},
		{
			name:    "app valid",
			kind:    "App",
			path:    "ws1/prv1/cls1/app1",
			wantErr: false,
		},
		{
			name:    "box valid",
			kind:    "Box",
			path:    "ws1/prv1/cls1/app1/box1",
			wantErr: false,
		},
		{
			name:    "unknown kind",
			kind:    "Unknown",
			path:    "ws1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSegmentCount(tt.kind, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSegmentCount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSegmentLabels(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid lowercase",
			path:    "ws1/prv1/cls1",
			wantErr: false,
		},
		{
			name:    "valid with hyphens",
			path:    "ws-1/prv-1/cls-1",
			wantErr: false,
		},
		{
			name:    "invalid uppercase",
			path:    "WS1/prv1/cls1",
			wantErr: true,
		},
		{
			name:    "invalid underscore",
			path:    "ws_1/prv1/cls1",
			wantErr: true,
		},
		{
			name:    "invalid starts with hyphen",
			path:    "-ws1/prv1/cls1",
			wantErr: true,
		},
		{
			name:    "invalid ends with hyphen",
			path:    "ws1-/prv1/cls1",
			wantErr: true,
		},
		{
			name:    "invalid empty segment",
			path:    "ws1//cls1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSegmentLabels(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSegmentLabels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildFQN(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		parentPath string
		objName    string
		want       FQN
		wantErr    bool
	}{
		{
			name:       "workspace",
			kind:       "Workspace",
			parentPath: "",
			objName:    "ws1",
			want:       "ws1",
			wantErr:    false,
		},
		{
			name:       "provider",
			kind:       "Provider",
			parentPath: "ws1",
			objName:    "prv1",
			want:       "ws1/prv1",
			wantErr:    false,
		},
		{
			name:       "cluster",
			kind:       "Cluster",
			parentPath: "ws1/prv1",
			objName:    "cls1",
			want:       "ws1/prv1/cls1",
			wantErr:    false,
		},
		{
			name:       "app",
			kind:       "App",
			parentPath: "ws1/prv1/cls1",
			objName:    "app1",
			want:       "ws1/prv1/cls1/app1",
			wantErr:    false,
		},
		{
			name:       "box",
			kind:       "Box",
			parentPath: "ws1/prv1/cls1/app1",
			objName:    "box1",
			want:       "ws1/prv1/cls1/app1/box1",
			wantErr:    false,
		},
		{
			name:       "provider without parent",
			kind:       "Provider",
			parentPath: "",
			objName:    "prv1",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "invalid name uppercase",
			kind:       "Workspace",
			parentPath: "",
			objName:    "WS1",
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildFQN(tt.kind, tt.parentPath, tt.objName)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildFQN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BuildFQN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractParentPath(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		annotations map[string]string
		want        string
		wantErr     bool
	}{
		{
			name:        "workspace has no parent",
			kind:        "Workspace",
			annotations: map[string]string{},
			want:        "",
			wantErr:     false,
		},
		{
			name: "provider with path",
			kind: "Provider",
			annotations: map[string]string{
				AnnotationPath: "ws1",
			},
			want:    "ws1",
			wantErr: false,
		},
		{
			name:        "provider without path",
			kind:        "Provider",
			annotations: map[string]string{},
			want:        "",
			wantErr:     true,
		},
		{
			name: "cluster with path",
			kind: "Cluster",
			annotations: map[string]string{
				AnnotationPath: "ws1/prv1",
			},
			want:    "ws1/prv1",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractParentPath(tt.kind, tt.annotations)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractParentPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractParentPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
