targetScope = 'subscription'

@minLength(1)
@maxLength(64)
@description('Name of the the environment which is used to generate a short unique hash used in all resources.')
param environmentName string

@minLength(1)
@description('Primary location for all resources')
param location string

@maxLength(80) // RG name max len is 90. Ensure buffer for suffixes.
param resourceGroupName string = ''

param keyVaultName string = ''
param logAnalyticsName string = ''
param applicationInsightsName string = ''
param applicationInsightsDashboardName string = ''
param storageAccountName string = ''
param aksName string = ''
param principalId string = deployer().objectId
param ingressServiceAccountNamespace string = 'traefik'
param ingressServiceAccountName string = 'traefik'
param aksSystemVmSize string = 'Standard_B4ms'
param aksSystemVmZones string = '1,2,3'
param aksUserVmSize string = 'Standard_B4ms'
param aksUserVmZones string = '1,2,3'

var abbrs = loadJsonContent('./abbreviations.json')

// tags that should be applied to all resources.
var tags = {
  // Tag all resources with the environment name.
  'azd-env-name': environmentName
}

// Generate a unique token to be used in naming resources.
// Remove linter suppression after using.
#disable-next-line no-unused-vars
var resourceToken = toLower(uniqueString(subscription().id, environmentName, location, rg.name))

// Compose AKS OIDC subject for ServiceAccount (system:serviceaccount:<namespace>:<name>)
var ingressServiceAccountSubject = 'system:serviceaccount:${ingressServiceAccountNamespace}:${ingressServiceAccountName}'

// Name of the service defined in azure.yaml
// A tag named azd-service-name with this value should be applied to the service host resource, such as:
//   Microsoft.Web/sites for appservice, function
// Example usage:
//   tags: union(tags, { 'azd-service-name': apiServiceName })
#disable-next-line no-unused-vars
var apiServiceName = 'python-api'

// Organize resources in a resource group
resource rg 'Microsoft.Resources/resourceGroups@2021-04-01' = {
  name: !empty(resourceGroupName) ? resourceGroupName : '${abbrs.resourcesResourceGroups}${environmentName}'
  location: location
  tags: tags
}

// Add resources to be provisioned below.
// A full example that leverages azd bicep modules can be seen in the todo-python-mongo template:
// https://github.com/Azure-Samples/todo-python-mongo/tree/main/infra

module keyVault './core/security/keyvault.bicep' = {
  name: 'keyVault'
  scope: rg
  params: {
    location: location
    tags: tags
    name: !empty(keyVaultName) ? keyVaultName : '${abbrs.keyVaultVaults}${resourceToken}'
    principalId: principalId
  }
}

module monitoring './core/monitor/monitoring.bicep' = {
  name: 'monitoring'
  scope: rg
  params: {
    location: location
    tags: tags
    logAnalyticsName: !empty(logAnalyticsName)
      ? logAnalyticsName
      : '${abbrs.operationalInsightsWorkspaces}${resourceToken}'
    applicationInsightsName: !empty(applicationInsightsName)
      ? applicationInsightsName
      : '${abbrs.insightsComponents}${resourceToken}'
    applicationInsightsDashboardName: !empty(applicationInsightsDashboardName)
      ? applicationInsightsDashboardName
      : '${abbrs.portalDashboards}${resourceToken}'
  }
}

module userIdentity './app/user-identity.bicep' = {
  name: 'userIdentity'
  scope: rg
  params: {
    location: location
    tags: tags
    name: '${abbrs.managedIdentityUserAssignedIdentities}${resourceToken}'
  }
}

module userIdentityFederation './app/user-identity-federation.bicep' = {
  name: 'userIdentityFederation'
  scope: rg
  params: {
    name: 'fic-ingress'
    userIdentityName: userIdentity.outputs.name
    issuerUrl: aks.outputs.oidcIssuerUrl
    subject: ingressServiceAccountSubject
    audience: 'api://AzureADTokenExchange'
  }
}

module storageAccount './app/storage-account.bicep' = {
  name: 'storageAccount'
  scope: rg
  params: {
    name: !empty(storageAccountName) ? storageAccountName : '${abbrs.storageStorageAccounts}${resourceToken}'
    location: location
    tags: tags
  }
}

module aks './app/aks.bicep' = {
  name: 'aks'
  scope: rg
  params: {
    name: !empty(aksName) ? aksName : '${abbrs.containerServiceManagedClusters}${resourceToken}'
    location: location
    tags: tags
    principalId: principalId
    nodeResourceGroupName: '${rg.name}_mc'
    logAnalyticsName: monitoring.outputs.logAnalyticsWorkspaceName
    keyVaultName: keyVault.outputs.name
    storageAccountName: storageAccount.outputs.name
    kubernetesVersion: '1.33'
    systemPoolVmSize: aksSystemVmSize
    systemPoolVmZones: split(aksSystemVmZones, ',')
    userPoolVmSize: aksUserVmSize
    userPoolVmZones: split(aksUserVmZones, ',')
  }
}

output AZURE_LOCATION string = location
output AZURE_TENANT_ID string = tenant().tenantId
output AZURE_SUBSCRIPTION_ID string = subscription().subscriptionId
output AZURE_RESOURCE_GROUP_NAME string = rg.name
output AZURE_AKS_CLUSTER_NAME string = aks.outputs.clusterName
output AZURE_AKS_PRINCIPAL_ID string = aks.outputs.principalId
output AZURE_AKS_OIDC_ISSUER_URL string = aks.outputs.oidcIssuerUrl
output AZURE_INGRESS_SERVICE_ACCOUNT_NAMESPACE string = ingressServiceAccountNamespace
output AZURE_INGRESS_SERVICE_ACCOUNT_NAME string = ingressServiceAccountName
output AZURE_INGRESS_SERVICE_ACCOUNT_PRINCIPAL_ID string = userIdentity.outputs.principalId
output AZURE_INGRESS_SERVICE_ACCOUNT_CLIENT_ID string = userIdentity.outputs.clientId
