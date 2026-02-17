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
			fqn:  "/ws/ws1",
			want: []string{"ws", "ws1"},
		},
		{
			name: "provider",
			fqn:  "/ws/ws1/prv/prv1",
			want: []string{"ws", "ws1", "prv", "prv1"},
		},
		{
			name: "cluster",
			fqn:  "/ws/ws1/prv/prv1/cls/cls1",
			want: []string{"ws", "ws1", "prv", "prv1", "cls", "cls1"},
		},
		{
			name: "app",
			fqn:  "/ws/ws1/prv/prv1/cls/cls1/app/app1",
			want: []string{"ws", "ws1", "prv", "prv1", "cls", "cls1", "app", "app1"},
		},
		{
			name: "box",
			fqn:  "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1",
			want: []string{"ws", "ws1", "prv", "prv1", "cls", "cls1", "app", "app1", "box", "box1"},
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
			fqn:  "/ws/ws1",
			want: "",
		},
		{
			name: "provider parent is workspace",
			fqn:  "/ws/ws1/prv/prv1",
			want: "/ws/ws1",
		},
		{
			name: "cluster parent is provider",
			fqn:  "/ws/ws1/prv/prv1/cls/cls1",
			want: "/ws/ws1/prv/prv1",
		},
		{
			name: "app parent is cluster",
			fqn:  "/ws/ws1/prv/prv1/cls/cls1/app/app1",
			want: "/ws/ws1/prv/prv1/cls/cls1",
		},
		{
			name: "box parent is app",
			fqn:  "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1",
			want: "/ws/ws1/prv/prv1/cls/cls1/app/app1",
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

func TestFQN_KindSegments(t *testing.T) {
	tests := []struct {
		name string
		fqn  FQN
		want [][2]string
	}{
		{
			name: "workspace",
			fqn:  "/ws/ws1",
			want: [][2]string{{"ws", "ws1"}},
		},
		{
			name: "provider",
			fqn:  "/ws/ws1/prv/prv1",
			want: [][2]string{{"ws", "ws1"}, {"prv", "prv1"}},
		},
		{
			name: "cluster",
			fqn:  "/ws/ws1/prv/prv1/cls/cls1",
			want: [][2]string{{"ws", "ws1"}, {"prv", "prv1"}, {"cls", "cls1"}},
		},
		{
			name: "empty",
			fqn:  "",
			want: nil,
		},
		{
			name: "odd segments",
			fqn:  "/ws/ws1/prv",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fqn.KindSegments()
			if len(got) != len(tt.want) {
				t.Errorf("KindSegments() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("KindSegments()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseResourceID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantFQN  FQN
		wantKind string
		wantErr  bool
	}{
		{
			name:     "workspace",
			id:       "/ws/ws1",
			wantFQN:  "/ws/ws1",
			wantKind: "Workspace",
			wantErr:  false,
		},
		{
			name:     "provider",
			id:       "/ws/ws1/prv/prv1",
			wantFQN:  "/ws/ws1/prv/prv1",
			wantKind: "Provider",
			wantErr:  false,
		},
		{
			name:     "cluster",
			id:       "/ws/ws1/prv/prv1/cls/cls1",
			wantFQN:  "/ws/ws1/prv/prv1/cls/cls1",
			wantKind: "Cluster",
			wantErr:  false,
		},
		{
			name:     "app",
			id:       "/ws/ws1/prv/prv1/cls/cls1/app/app1",
			wantFQN:  "/ws/ws1/prv/prv1/cls/cls1/app/app1",
			wantKind: "App",
			wantErr:  false,
		},
		{
			name:     "box",
			id:       "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1",
			wantFQN:  "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1",
			wantKind: "Box",
			wantErr:  false,
		},
		{
			name:     "missing leading slash",
			id:       "ws/ws1",
			wantFQN:  "",
			wantKind: "",
			wantErr:  true,
		},
		{
			name:     "odd segments",
			id:       "/ws/ws1/prv",
			wantFQN:  "",
			wantKind: "",
			wantErr:  true,
		},
		{
			name:     "unknown kind",
			id:       "/foo/bar",
			wantFQN:  "",
			wantKind: "",
			wantErr:  true,
		},
		{
			name:     "invalid name uppercase",
			id:       "/ws/WS1",
			wantFQN:  "",
			wantKind: "",
			wantErr:  true,
		},
		{
			name:     "invalid name with underscore",
			id:       "/ws/ws_1",
			wantFQN:  "",
			wantKind: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFQN, gotKind, err := ParseResourceID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResourceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFQN != tt.wantFQN {
				t.Errorf("ParseResourceID() FQN = %v, want %v", gotFQN, tt.wantFQN)
			}
			if gotKind != tt.wantKind {
				t.Errorf("ParseResourceID() Kind = %v, want %v", gotKind, tt.wantKind)
			}
		})
	}
}

func TestValidateResourceID(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		expectedKind string
		expectedName string
		wantFQN      FQN
		wantErr      bool
	}{
		{
			name:         "workspace valid",
			id:           "/ws/ws1",
			expectedKind: "Workspace",
			expectedName: "ws1",
			wantFQN:      "/ws/ws1",
			wantErr:      false,
		},
		{
			name:         "provider valid",
			id:           "/ws/ws1/prv/prv1",
			expectedKind: "Provider",
			expectedName: "prv1",
			wantFQN:      "/ws/ws1/prv/prv1",
			wantErr:      false,
		},
		{
			name:         "kind mismatch",
			id:           "/ws/ws1/prv/prv1",
			expectedKind: "Cluster",
			expectedName: "prv1",
			wantFQN:      "",
			wantErr:      true,
		},
		{
			name:         "name mismatch",
			id:           "/ws/ws1",
			expectedKind: "Workspace",
			expectedName: "ws2",
			wantFQN:      "",
			wantErr:      true,
		},
		{
			name:         "provider with invalid parent chain",
			id:           "/prv/prv1",
			expectedKind: "Provider",
			expectedName: "prv1",
			wantFQN:      "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFQN, err := ValidateResourceID(tt.id, tt.expectedKind, tt.expectedName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResourceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFQN != tt.wantFQN {
				t.Errorf("ValidateResourceID() = %v, want %v", gotFQN, tt.wantFQN)
			}
		})
	}
}

func TestBuildResourceID(t *testing.T) {
	tests := []struct {
		name     string
		parentID FQN
		kind     string
		objName  string
		want     FQN
		wantErr  bool
	}{
		{
			name:     "workspace",
			parentID: "",
			kind:     "Workspace",
			objName:  "ws1",
			want:     "/ws/ws1",
			wantErr:  false,
		},
		{
			name:     "provider",
			parentID: "/ws/ws1",
			kind:     "Provider",
			objName:  "prv1",
			want:     "/ws/ws1/prv/prv1",
			wantErr:  false,
		},
		{
			name:     "cluster",
			parentID: "/ws/ws1/prv/prv1",
			kind:     "Cluster",
			objName:  "cls1",
			want:     "/ws/ws1/prv/prv1/cls/cls1",
			wantErr:  false,
		},
		{
			name:     "app",
			parentID: "/ws/ws1/prv/prv1/cls/cls1",
			kind:     "App",
			objName:  "app1",
			want:     "/ws/ws1/prv/prv1/cls/cls1/app/app1",
			wantErr:  false,
		},
		{
			name:     "box",
			parentID: "/ws/ws1/prv/prv1/cls/cls1/app/app1",
			kind:     "Box",
			objName:  "box1",
			want:     "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1",
			wantErr:  false,
		},
		{
			name:     "workspace with parent error",
			parentID: "/ws/ws1",
			kind:     "Workspace",
			objName:  "ws2",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "provider without parent error",
			parentID: "",
			kind:     "Provider",
			objName:  "prv1",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "invalid name uppercase",
			parentID: "",
			kind:     "Workspace",
			objName:  "WS1",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildResourceID(tt.parentID, tt.kind, tt.objName)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildResourceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BuildResourceID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractResourceID(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		objName     string
		annotations map[string]string
		want        FQN
		wantErr     bool
	}{
		{
			name:    "workspace valid",
			kind:    "Workspace",
			objName: "ws1",
			annotations: map[string]string{
				AnnotationID: "/ws/ws1",
			},
			want:    "/ws/ws1",
			wantErr: false,
		},
		{
			name:    "provider valid",
			kind:    "Provider",
			objName: "prv1",
			annotations: map[string]string{
				AnnotationID: "/ws/ws1/prv/prv1",
			},
			want:    "/ws/ws1/prv/prv1",
			wantErr: false,
		},
		{
			name:        "missing annotation",
			kind:        "Provider",
			objName:     "prv1",
			annotations: map[string]string{},
			want:        "",
			wantErr:     true,
		},
		{
			name:    "name mismatch",
			kind:    "Workspace",
			objName: "ws1",
			annotations: map[string]string{
				AnnotationID: "/ws/ws2",
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractResourceID(tt.kind, tt.objName, tt.annotations)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractResourceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractResourceID() = %v, want %v", got, tt.want)
			}
		})
	}
}
