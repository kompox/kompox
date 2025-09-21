package kube

// Centralized label and annotation keys used by the kube adapter.
// Keep these constants stable; changes are API-visible in clusters.
const (
	// K4xDomain is the namespace domain for all Kompox (K4x) custom labels and annotations.
	K4xDomain = "kompox.dev"

	LabelAppK8sName      = "app.kubernetes.io/name"
	LabelAppK8sInstance  = "app.kubernetes.io/instance"
	LabelAppK8sManagedBy = "app.kubernetes.io/managed-by"
	LabelAppK8sComponent = "app.kubernetes.io/component"

	LabelAppSelector               = "app"
	LabelK4xAppInstanceHash        = K4xDomain + "/app-instance-hash"
	LabelK4xAppIDHash              = K4xDomain + "/app-id-hash"
	LabelK4xComposeServiceHeadless = K4xDomain + "/compose-service-headless"
	LabelK4xNodePool               = K4xDomain + "/node-pool"
	LabelK4xNodeZone               = K4xDomain + "/node-zone"

	AnnotationK4xApp               = K4xDomain + "/app"
	AnnotationK4xProviderDriver    = K4xDomain + "/provider-driver"
	AnnotationK4xComposeSecretHash = K4xDomain + "/compose-secret-hash"
)
