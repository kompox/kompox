package model

import "errors"

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrWorkspaceInvalid  = errors.New("workspace invalid")
	ErrProviderNotFound  = errors.New("provider not found")
	ErrProviderInvalid   = errors.New("provider invalid")
	ErrClusterNotFound   = errors.New("cluster not found")
	ErrClusterInvalid    = errors.New("cluster invalid")
	ErrAppNotFound       = errors.New("app not found")
	ErrAppInvalid        = errors.New("app invalid")

	// ErrClusterProtected indicates that the operation is blocked by protection policy.
	ErrClusterProtected = errors.New("cluster operation blocked by protection policy")
)
