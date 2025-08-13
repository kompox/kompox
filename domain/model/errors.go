package model

import "errors"

var (
	ErrServiceNotFound  = errors.New("service not found")
	ErrServiceInvalid   = errors.New("service invalid")
	ErrProviderNotFound = errors.New("provider not found")
	ErrProviderInvalid  = errors.New("provider invalid")
	ErrClusterNotFound  = errors.New("cluster not found")
	ErrClusterInvalid   = errors.New("cluster invalid")
	ErrAppNotFound      = errors.New("app not found")
	ErrAppInvalid       = errors.New("app invalid")
)
