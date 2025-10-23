package model

import "fmt"

// CheckProvisioningProtection validates if provisioning operations are allowed.
// Returns an error if the operation is blocked by the protection level.
func (c *Cluster) CheckProvisioningProtection(operation string) error {
	if c.Protection == nil {
		return nil
	}
	level := c.Protection.Provisioning
	if level == ProtectionNone || level == "" {
		return nil
	}
	return fmt.Errorf("%w: provisioning is %q, cannot perform %s (set to 'none' to unlock)", ErrClusterProtected, level, operation)
}

// CheckInstallationProtection validates if installation operations are allowed.
// Returns an error if the operation is blocked by the protection level.
// For update operations, pass isUpdate=true to also check for readOnly protection.
func (c *Cluster) CheckInstallationProtection(operation string, isUpdate bool) error {
	if c.Protection == nil {
		return nil
	}
	level := c.Protection.Installation
	if level == ProtectionNone || level == "" {
		return nil
	}
	if level == ProtectionReadOnly {
		return fmt.Errorf("%w: installation is %q, cannot perform %s (set to 'none' to unlock)", ErrClusterProtected, level, operation)
	}
	if level == ProtectionCannotDelete && !isUpdate {
		return fmt.Errorf("%w: installation is %q, cannot perform %s (set to 'none' to unlock)", ErrClusterProtected, level, operation)
	}
	return nil
}
