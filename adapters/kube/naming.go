package kube

// SecretEnvBaseName returns `<appName>-<componentName>-<containerName>-base`.
// Used for Compose env_file aggregated environment Secret (optional).
func SecretEnvBaseName(appName, componentName, containerName string) string {
	return appName + "-" + componentName + "-" + containerName + "-base"
}

// SecretEnvOverrideName returns `<appName>-<componentName>-<containerName>-override`.
// Used for CLI override environment Secret (optional; created by `kompoxops app env`).
func SecretEnvOverrideName(appName, componentName, containerName string) string {
	return appName + "-" + componentName + "-" + containerName + "-override"
}

// SecretPullName returns `<appName>-<componentName>--pull`.
// Used for registry auth Secret (kubernetes.io/dockerconfigjson) created by CLI `kompoxops app pull`.
func SecretPullName(appName, componentName string) string {
	return appName + "-" + componentName + "--pull"
}

// ConfigMapName returns `<appName>-<componentName>--cfg-<configName>`.
// Used for ConfigMap resource generated from Compose top-level configs.
func ConfigMapName(appName, componentName, configName string) string {
	return appName + "-" + componentName + "--cfg-" + configName
}

// ConfigSecretName returns `<appName>-<componentName>--sec-<secretName>`.
// Used for Secret resource generated from Compose top-level secrets.
func ConfigSecretName(appName, componentName, secretName string) string {
	return appName + "-" + componentName + "--sec-" + secretName
}

// ConfigMapVolumeName returns `cfg-<configName>`.
// Used for volume name when mounting a ConfigMap as single file.
func ConfigMapVolumeName(configName string) string {
	return "cfg-" + configName
}

// ConfigSecretVolumeName returns `sec-<secretName>`.
// Used for volume name when mounting a Secret as single file.
func ConfigSecretVolumeName(secretName string) string {
	return "sec-" + secretName
}
