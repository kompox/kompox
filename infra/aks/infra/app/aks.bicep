param name string

param location string = resourceGroup().location

param tags object = {}

param containerRegistryName string = ''

param logAnalyticsName string = ''

param storageAccountName string = ''

param keyVaultName string

@allowed(['Free', 'Paid', 'Standard'])
param sku string = 'Free'

@allowed(['azure', 'kubenet', 'none'])
param networkPlugin string = 'azure'

@allowed(['azure', 'calico'])
param networkPolicy string = 'azure'

param dnsPrefix string = ''

param nodeResourceGroupName string = ''

@allowed(['CostOptimised', 'Standard', 'HighSpec', 'Custom'])
param systemPoolType string = 'CostOptimised'

param systemPoolConfig object = {}

param principalId string = deployer().objectId

param kubernetesVersion string = '1.29'

param aadTenantId string = tenant().tenantId

var nodePoolPresets = {
  CostOptimised: {
    vmSize: 'Standard_B4ms'
    count: 1
    minCount: 1
    maxCount: 3
    enableAutoScaling: true
    availabilityZones: []
  }
  Standard: {
    vmSize: 'Standard_DS2_v2'
    count: 3
    minCount: 3
    maxCount: 5
    enableAutoScaling: true
    availabilityZones: [
      '1'
      '2'
      '3'
    ]
  }
  HighSpec: {
    vmSize: 'Standard_D4s_v3'
    count: 3
    minCount: 3
    maxCount: 5
    enableAutoScaling: true
    availabilityZones: [
      '1'
      '2'
      '3'
    ]
  }
}

var systemPoolSpec = !empty(systemPoolConfig) ? systemPoolConfig : nodePoolPresets[systemPoolType]

var nodePoolBase = {
  osType: 'Linux'
  maxPods: 30
  type: 'VirtualMachineScaleSets'
  upgradeSettings: {
    maxSurge: '33%'
  }
}

resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2021-12-01-preview' existing = if (!empty(logAnalyticsName)) {
  name: logAnalyticsName
}

resource aks 'Microsoft.ContainerService/managedClusters@2025-05-02-preview' = {
  name: name
  location: location
  tags: tags
  identity: {
    type: 'SystemAssigned'
  }
  sku: {
    name: 'Base'
    tier: sku
  }
  properties: {
    nodeResourceGroup: !empty(nodeResourceGroupName) ? nodeResourceGroupName : 'rg-mc-${name}'
    kubernetesVersion: kubernetesVersion
    dnsPrefix: empty(dnsPrefix) ? '${name}-dns' : dnsPrefix
    enableRBAC: true
    aadProfile: {
      managed: true
      enableAzureRBAC: true
      tenantID: aadTenantId
    }
    agentPoolProfiles: [
      union({ name: 'npsystem', mode: 'System' }, nodePoolBase, systemPoolSpec)
    ]
    networkProfile: {
      loadBalancerSku: 'standard'
      networkPlugin: networkPlugin
      networkPolicy: networkPolicy
    }
    addonProfiles: {
      azurepolicy: {
        enabled: true
        config: { version: 'v2' }
      }
      azureKeyvaultSecretsProvider: {
        enabled: true
        config: { enableSecretRotation: 'true', rotationPollInterval: '2m' }
      }
      omsagent: {
        enabled: true
        config: empty(logAnalyticsName) ? {} : { logAnalyticsWorkspaceResourceID: logAnalytics.id }
      }
    }
    ingressProfile: {
      webAppRouting: {
        enabled: true
      }
    }
    oidcIssuerProfile: {
      enabled: true
    }
    securityProfile: {
      workloadIdentity: {
        enabled: true
      }
    }
  }
}

// Diagnostic settings to send AKS logs and metrics to a storage account
resource storageAccount 'Microsoft.Storage/storageAccounts@2023-01-01' existing = if (!empty(storageAccountName)) {
  name: storageAccountName
}

resource diagnostics 'Microsoft.Insights/diagnosticSettings@2021-05-01-preview' = if (!empty(storageAccountName)) {
  name: 'diagnostics'
  scope: aks
  properties: {
    storageAccountId: storageAccount.id
    logs: [
      {
        category: 'cluster-autoscaler'
        enabled: true
      }
      {
        category: 'guard'
        enabled: true
      }
    ]
    metrics: [
      {
        category: 'AllMetrics'
        enabled: true
      }
    ]
  }
}

// Role assignment for AKS cluster admin
var aksClusterAdminRole = subscriptionResourceId(
  'Microsoft.Authorization/roleDefinitions',
  'b1ff04bb-8a4e-4dc4-8eb5-8693973ce19b'
)

resource aksRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: aks
  name: guid(subscription().id, resourceGroup().id, principalId, aksClusterAdminRole)
  properties: {
    roleDefinitionId: aksClusterAdminRole
    principalId: principalId
  }
}

// Role assignment for Azure Key Vault access
resource keyVault 'Microsoft.KeyVault/vaults@2023-07-01' existing = if (!empty(keyVaultName)) {
  name: keyVaultName
}

var keyVaultSecretsUserRole = subscriptionResourceId(
  'Microsoft.Authorization/roleDefinitions',
  '4633458b-17de-408a-b874-0445c86b69e6' // Key Vault Secrets User
)

var keyVaultCertificateUserRole = subscriptionResourceId(
  'Microsoft.Authorization/roleDefinitions',
  'db79e9a7-68ee-4b58-9aeb-b90e7c24fcba' // Key Vault Certificate User
)

resource aksKeyVaultRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (!empty(keyVaultName)) {
  scope: keyVault
  name: guid(keyVault.id, aks.id, keyVaultSecretsUserRole)
  properties: {
    roleDefinitionId: keyVaultSecretsUserRole
    principalId: aks.identity.principalId
  }
}

resource aksKeyVaultCertRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (!empty(keyVaultName)) {
  scope: keyVault
  name: guid(keyVault.id, aks.id, keyVaultCertificateUserRole)
  properties: {
    roleDefinitionId: keyVaultCertificateUserRole
    principalId: aks.identity.principalId
  }
}

// Role assignment for ACR pull access
resource containerRegistry 'Microsoft.ContainerRegistry/registries@2023-07-01' existing = if (!empty(containerRegistryName)) {
  name: containerRegistryName
}

var acrPullRole = subscriptionResourceId(
  'Microsoft.Authorization/roleDefinitions',
  '7f951dda-4ed3-4680-a7ca-43fe172d538d' // AcrPull
)

resource aksAcrPullRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (!empty(containerRegistryName)) {
  scope: containerRegistry
  name: guid(containerRegistry.id, aks.id, acrPullRole)
  properties: {
    roleDefinitionId: acrPullRole
    principalId: aks.identity.principalId
  }
}

output clusterName string = aks.name
output principalId string = aks.identity.principalId
output oidcIssuerUrl string = aks.properties.oidcIssuerProfile.issuerURL
