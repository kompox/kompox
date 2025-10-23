package model

import "fmt"

// CheckProvisioningProtection validates if provisioning operations are allowed.
// Returns an error if the operation is blocked by the protection level.
// opType should be OpCreate, OpUpdate, or OpDelete.
func (c *Cluster) CheckProvisioningProtection(opType ClusterOperationType) error {
	if c.Protection == nil {
		return nil
	}
	level := c.Protection.Provisioning
	if level == ProtectionNone || level == "" {
		return nil
	}

	// readOnly: block all operations except initial creation
	if level == ProtectionReadOnly {
		if opType == OpUpdate {
			return fmt.Errorf("%w: provisioning is %q, cannot perform %s operation (set to 'none' or 'cannotDelete' to unblock)", ErrClusterProtected, level, opType)
		}
		if opType == OpDelete {
			return fmt.Errorf("%w: provisioning is %q, cannot perform %s operation (set to 'none' to unblock)", ErrClusterProtected, level, opType)
		}
	}

	// cannotDelete: block delete operations only, allow updates
	if level == ProtectionCannotDelete && opType == OpDelete {
		return fmt.Errorf("%w: provisioning is %q, cannot perform %s operation (set to 'none' to unblock)", ErrClusterProtected, level, opType)
	}

	return nil
}

// CheckInstallationProtection validates if installation operations are allowed.
// Returns an error if the operation is blocked by the protection level.
// opType should be OpCreate, OpUpdate, or OpDelete.
func (c *Cluster) CheckInstallationProtection(opType ClusterOperationType) error {
	if c.Protection == nil {
		return nil
	}
	level := c.Protection.Installation
	if level == ProtectionNone || level == "" {
		return nil
	}

	// readOnly: block all operations except initial creation
	if level == ProtectionReadOnly {
		if opType == OpUpdate {
			return fmt.Errorf("%w: installation is %q, cannot perform %s operation (set to 'none' or 'cannotDelete' to unblock)", ErrClusterProtected, level, opType)
		}
		if opType == OpDelete {
			return fmt.Errorf("%w: installation is %q, cannot perform %s operation (set to 'none' to unblock)", ErrClusterProtected, level, opType)
		}
	}

	// cannotDelete: block delete operations only, allow updates
	if level == ProtectionCannotDelete && opType == OpDelete {
		return fmt.Errorf("%w: installation is %q, cannot perform %s operation (set to 'none' to unblock)", ErrClusterProtected, level, opType)
	}

	return nil
}
