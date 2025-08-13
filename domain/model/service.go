package model

import "time"

// Service represents a deployable logical service.
type Service struct {
	ID         string
	Name       string
	ProviderID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
