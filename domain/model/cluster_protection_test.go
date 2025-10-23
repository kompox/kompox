package model

import (
	"errors"
	"testing"
)

func TestCluster_CheckProvisioningProtection(t *testing.T) {
	tests := []struct {
		name      string
		cluster   *Cluster
		operation string
		wantErr   bool
	}{
		{
			name:      "nil protection allows operation",
			cluster:   &Cluster{Protection: nil},
			operation: "deprovision",
			wantErr:   false,
		},
		{
			name:      "none allows operation",
			cluster:   &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionNone}},
			operation: "deprovision",
			wantErr:   false,
		},
		{
			name:      "empty string allows operation",
			cluster:   &Cluster{Protection: &ClusterProtection{Provisioning: ""}},
			operation: "deprovision",
			wantErr:   false,
		},
		{
			name:      "cannotDelete blocks deprovision",
			cluster:   &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionCannotDelete}},
			operation: "deprovision",
			wantErr:   true,
		},
		{
			name:      "readOnly blocks deprovision",
			cluster:   &Cluster{Protection: &ClusterProtection{Provisioning: ProtectionReadOnly}},
			operation: "deprovision",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.CheckProvisioningProtection(tt.operation)
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
		name      string
		cluster   *Cluster
		operation string
		isUpdate  bool
		wantErr   bool
	}{
		{
			name:      "nil protection allows operation",
			cluster:   &Cluster{Protection: nil},
			operation: "uninstall",
			isUpdate:  false,
			wantErr:   false,
		},
		{
			name:      "none allows operation",
			cluster:   &Cluster{Protection: &ClusterProtection{Installation: ProtectionNone}},
			operation: "uninstall",
			isUpdate:  false,
			wantErr:   false,
		},
		{
			name:      "cannotDelete blocks uninstall",
			cluster:   &Cluster{Protection: &ClusterProtection{Installation: ProtectionCannotDelete}},
			operation: "uninstall",
			isUpdate:  false,
			wantErr:   true,
		},
		{
			name:      "cannotDelete allows update",
			cluster:   &Cluster{Protection: &ClusterProtection{Installation: ProtectionCannotDelete}},
			operation: "install",
			isUpdate:  true,
			wantErr:   false,
		},
		{
			name:      "readOnly blocks uninstall",
			cluster:   &Cluster{Protection: &ClusterProtection{Installation: ProtectionReadOnly}},
			operation: "uninstall",
			isUpdate:  false,
			wantErr:   true,
		},
		{
			name:      "readOnly blocks update",
			cluster:   &Cluster{Protection: &ClusterProtection{Installation: ProtectionReadOnly}},
			operation: "install",
			isUpdate:  true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.CheckInstallationProtection(tt.operation, tt.isUpdate)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckInstallationProtection() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrClusterProtected) {
				t.Errorf("CheckInstallationProtection() error = %v, should wrap ErrClusterProtected", err)
			}
		})
	}
}
