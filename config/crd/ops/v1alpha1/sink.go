package v1alpha1

import (
	"fmt"
)

// Sink provides an immutable in-memory index for CRD documents.
// It uses FQN as the primary key for efficient lookups.
// Once created, the sink cannot be modified (immutable).
type Sink struct {
	workspaces map[FQN]*Workspace
	providers  map[FQN]*Provider
	clusters   map[FQN]*Cluster
	apps       map[FQN]*App
	boxes      map[FQN]*Box
}

// NewSink validates documents and creates an immutable Sink.
// Returns an error if validation fails.
func NewSink(documents []Document) (*Sink, error) {
	// Validate documents
	validated := Validate(documents)
	if validated.HasErrors() {
		return nil, fmt.Errorf("validation errors: %v", validated.Errors)
	}

	// Build immutable sink
	sink := &Sink{
		workspaces: make(map[FQN]*Workspace),
		providers:  make(map[FQN]*Provider),
		clusters:   make(map[FQN]*Cluster),
		apps:       make(map[FQN]*App),
		boxes:      make(map[FQN]*Box),
	}

	// Populate sink with valid documents
	for _, doc := range validated.ValidDocuments {
		switch doc.Kind {
		case "Workspace":
			if ws, ok := doc.Object.(*Workspace); ok {
				sink.workspaces[doc.FQN] = ws
			}
		case "Provider":
			if prv, ok := doc.Object.(*Provider); ok {
				sink.providers[doc.FQN] = prv
			}
		case "Cluster":
			if cls, ok := doc.Object.(*Cluster); ok {
				sink.clusters[doc.FQN] = cls
			}
		case "App":
			if app, ok := doc.Object.(*App); ok {
				sink.apps[doc.FQN] = app
			}
		case "Box":
			if box, ok := doc.Object.(*Box); ok {
				sink.boxes[doc.FQN] = box
			}
		}
	}

	return sink, nil
}

// GetWorkspace retrieves a copy of a Workspace by FQN.
func (s *Sink) GetWorkspace(fqn FQN) (*Workspace, bool) {
	ws, ok := s.workspaces[fqn]
	if !ok {
		return nil, false
	}
	copy := *ws
	return &copy, true
}

// GetProvider retrieves a copy of a Provider by FQN.
func (s *Sink) GetProvider(fqn FQN) (*Provider, bool) {
	prv, ok := s.providers[fqn]
	if !ok {
		return nil, false
	}
	copy := *prv
	return &copy, true
}

// GetCluster retrieves a copy of a Cluster by FQN.
func (s *Sink) GetCluster(fqn FQN) (*Cluster, bool) {
	cls, ok := s.clusters[fqn]
	if !ok {
		return nil, false
	}
	copy := *cls
	return &copy, true
}

// GetApp retrieves a copy of an App by FQN.
func (s *Sink) GetApp(fqn FQN) (*App, bool) {
	app, ok := s.apps[fqn]
	if !ok {
		return nil, false
	}
	copy := *app
	return &copy, true
}

// GetBox retrieves a copy of a Box by FQN.
func (s *Sink) GetBox(fqn FQN) (*Box, bool) {
	box, ok := s.boxes[fqn]
	if !ok {
		return nil, false
	}
	copy := *box
	return &copy, true
}

// ListWorkspaces returns copies of all workspaces.
func (s *Sink) ListWorkspaces() []*Workspace {
	list := make([]*Workspace, 0, len(s.workspaces))
	for _, ws := range s.workspaces {
		copy := *ws
		list = append(list, &copy)
	}
	return list
}

// ListProviders returns copies of all providers.
func (s *Sink) ListProviders() []*Provider {
	list := make([]*Provider, 0, len(s.providers))
	for _, prv := range s.providers {
		copy := *prv
		list = append(list, &copy)
	}
	return list
}

// ListClusters returns copies of all clusters.
func (s *Sink) ListClusters() []*Cluster {
	list := make([]*Cluster, 0, len(s.clusters))
	for _, cls := range s.clusters {
		copy := *cls
		list = append(list, &copy)
	}
	return list
}

// ListApps returns copies of all apps.
func (s *Sink) ListApps() []*App {
	list := make([]*App, 0, len(s.apps))
	for _, app := range s.apps {
		copy := *app
		list = append(list, &copy)
	}
	return list
}

// ListBoxes returns copies of all boxes.
func (s *Sink) ListBoxes() []*Box {
	list := make([]*Box, 0, len(s.boxes))
	for _, box := range s.boxes {
		copy := *box
		list = append(list, &copy)
	}
	return list
}

// Count returns the total number of documents in the sink.
func (s *Sink) Count() int {
	return len(s.workspaces) + len(s.providers) + len(s.clusters) + len(s.apps) + len(s.boxes)
}
