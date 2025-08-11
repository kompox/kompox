package cluster

// Provider abstracts provider-specific behavior (identifier, hooks, etc.).
// Implementations live under cluster/providers/<name> and should return a
// provider identifier such as "aks".
type Provider interface {
	// ID returns the provider identifier (e.g., "aks").
	ID() string
}
