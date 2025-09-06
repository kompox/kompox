package box

import (
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

const (
	BoxResourceName            = "kompox-box"
	BoxContainerName           = "box"
	BoxUserName                = "kompox"
	LabelComponent             = "app.kubernetes.io/component"
	LabelComponentValue        = "kompox-box"
	LabelBox                   = "kompox.dev/box"
	LabelBoxValue              = "true"
	LabelBoxSelector           = LabelBox + "=" + LabelBoxValue
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
