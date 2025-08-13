package model

import "errors"

var (
	ErrServiceNotFound = errors.New("service not found")
	ErrServiceInvalid  = errors.New("service invalid")
)
