package box

import (
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

const (
	BoxContainerName           = "box"
	BoxUserName                = "kompox"
	AnnotationBoxSSHPubkeyHash = "kompox.dev/box-ssh-pubkey-hash"
)

// Repos holds repositories required for box operations.
type Repos struct {
	Service  domain.ServiceRepository
	Provider domain.ProviderRepository
	Cluster  domain.ClusterRepository
	App      domain.AppRepository
}

// UseCase wires dependencies for box operations.
type UseCase struct {
	Repos      *Repos
	VolumePort model.VolumePort
}
