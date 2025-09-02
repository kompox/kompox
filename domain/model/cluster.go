package model

import (
	"time"
)

// Cluster represents a Kubernetes cluster resource.
type Cluster struct {
	ID         string
	Name       string
	ProviderID string // references Provider
	Existing   bool
	Ingress    *ClusterIngress
	Settings   map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ClusterIngress defines cluster-level ingress configuration.
type ClusterIngress struct {
	Namespace      string
	Controller     string
	ServiceAccount string
	// Domain is the default DNS domain used to generate app ingress hosts.
	// This value is sourced from configuration at cluster.ingress.domain.
	Domain string
	// CertResolver selects the Traefik ACME resolver (e.g., "staging", "production").
	CertResolver string
	// CertEmail is the email address used for ACME account registration.
	CertEmail string
	// Certificates are static TLS certificates to be made available to the ingress controller.
	Certificates []ClusterIngressCertificate
}

// ClusterIngressCertificate represents a static certificate reference.
// Name is an arbitrary identifier; it determines the Kubernetes TLS Secret name as "tls-" + Name.
// Source is a provider-specific locator. For AKS, a Key Vault secret URL is supported.
type ClusterIngressCertificate struct {
	Name   string
	Source string
}
