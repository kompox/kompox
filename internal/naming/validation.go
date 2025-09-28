package naming

import (
	"fmt"
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	volumeNameMaxLength   = 16
	diskNameMaxLength     = 24
	snapshotNameMaxLength = 24
)

func validateDNS1123Label(name string, maximum int, labelKind string) error {
	if name == "" {
		return fmt.Errorf("%s name must not be empty", labelKind)
	}
	if len(name) > maximum {
		return fmt.Errorf("%s name exceeds %d characters", labelKind, maximum)
	}
	if errs := utilvalidation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("invalid %s name: %s", labelKind, strings.Join(errs, ", "))
	}
	return nil
}

func ValidateVolumeName(name string) error {
	return validateDNS1123Label(name, volumeNameMaxLength, "volume")
}

func ValidateDiskName(name string) error {
	return validateDNS1123Label(name, diskNameMaxLength, "disk")
}

func ValidateSnapshotName(name string) error {
	return validateDNS1123Label(name, snapshotNameMaxLength, "snapshot")
}
