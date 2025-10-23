package model

import (
	"errors"
	"testing"
)

func TestCluster_CheckProvisioningProtection(t *testing.T) {
	tests := []struct {
		name    string
		cluster *Cluster
		opType  ClusterOperationType
		wantErr bool
	}{
		{
			name:    "nil protection allows all operations",
			cluster: &Cluster{Protection: nil},
			opType:  OpDelete,
			wantErr: false,
		},
		{
			name:    "none allows all operations",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionNone}},
			opType:  OpDelete,
			wantErr: false,
		},
		{
			name:    "empty string allows all operations",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ""}},
			opType:  OpDelete,
			wantErr: false,
		},
		{
			name:    "cannotDelete blocks delete",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionCannotDelete}},
			opType:  OpDelete,
			wantErr: true,
		},
		{
			name:    "cannotDelete allows update",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionCannotDelete}},
			opType:  OpUpdate,
			wantErr: false,
		},
		{
			name:    "cannotDelete allows create",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionCannotDelete}},
			opType:  OpCreate,
			wantErr: false,
		},
		{
			name:    "readOnly blocks delete",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionReadOnly}},
			opType:  OpDelete,
			wantErr: true,
		},
		{
			name:    "readOnly blocks update",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionReadOnly}},
			opType:  OpUpdate,
			wantErr: true,
		},
		{
			name:    "readOnly allows create",
			cluster: &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionReadOnly}},
			opType:  OpCreate,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.CheckProvisioningProtection(tt.opType)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckProvisioningProtection() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrClusterProtected) {
				t.Errorf("CheckProvisioningProtection() error = %v, should wrap ErrClusterProtected", err)
			}
		})
	}
}

func TestCluster_CheckInstallationProtection(t *testing.T) {
	tests := []struct {
		name    string
		cluster *Cluster
		opType  ClusterOperationType
		wantErr bool
	}{
		{
			name:    "nil protection allows all operations",
			cluster: &Cluster{Protection: nil},
			opType:  OpDelete,
			wantErr: false,
		},
		{
			name:    "none allows all operations",
			cluster: &Cluster{Protection: &ClusterProtection{Installation: ProtectionNone}},
			opType:  OpDelete,
			wantErr: false,
		},
		{
			name:    "cannotDelete blocks delete",
			cluster: &Cluster{Protection: &ClusterProtection{Installation: ProtectionCannotDelete}},
			opType:  OpDelete,
			wantErr: true,
		},
		{
			name:    "cannotDelete allows update",
			cluster: &Cluster{Protection: &ClusterProtection{Installation: ProtectionCannotDelete}},
			opType:  OpUpdate,
			wantErr: false,
		},
		{
			name:    "cannotDelete allows create",
			cluster: &Cluster{Protection: &ClusterProtection{Installation: ProtectionCannotDelete}},
			opType:  OpCreate,
			wantErr: false,
		},
		{
			name:    "readOnly blocks delete",
			cluster: &Cluster{Protection: &ClusterProtection{Installation: ProtectionReadOnly}},
			opType:  OpDelete,
			wantErr: true,
		},
		{
			name:    "readOnly blocks update",
			cluster: &Cluster{Protection: &ClusterProtection{Installation: ProtectionReadOnly}},
			opType:  OpUpdate,
			wantErr: true,
		},
		{
			name:    "readOnly allows create",
			cluster: &Cluster{Protection: &ClusterProtection{Installation: ProtectionReadOnly}},
			opType:  OpCreate,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.CheckInstallationProtection(tt.opType)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckInstallationProtection() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrClusterProtected) {
				t.Errorf("CheckInstallationProtection() error = %v, should wrap ErrClusterProtected", err)
			}
		})
	}
}
