package model

import "time"

// Workspace represents a deployable logical workspace.
type Workspace struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
