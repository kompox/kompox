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

@allowed(['', 'overlay'])
param networkPluginMode string = 'overlay'

@allowed(['azure', 'calico'])
param networkPolicy string = 'azure'

param dnsPrefix string = ''

param nodeResourceGroupName string = ''

param principalId string = deployer().objectId

param kubernetesVersion string = '1.33'

param aadTenantId string = tenant().tenantId

param nodePoolMaxPods int = 250

param systemPoolConfig object = {}
param systemPoolVmSize string = 'Standard_D2lds_v6'
param systemPoolVmDiskType string = 'Ephemeral'
param systemPoolVmDiskSizeGB string = '110'
param systemPoolVmPriority string = 'Regular'
param systemPoolVmZones array = []
param userPoolConfig object = {}
param userPoolVmSize string = 'Standard_D2lds_v6'
param userPoolVmDiskType string = 'Ephemeral'
param userPoolVmDiskSizeGB string = '110'
param userPoolVmPriority string = 'Regular'
param userPoolVmZones array = ['1', '2', '3']

var systemPoolBase = !empty(systemPoolConfig)
  ? systemPoolConfig
  : {
      vmSize: systemPoolVmSize
      availabilityZones: systemPoolVmZones
      osType: 'Linux'
      osDiskType: systemPoolVmDiskType
      osDiskSizeGB: int(systemPoolVmDiskSizeGB)
      maxPods: nodePoolMaxPods
      type: 'VirtualMachineScaleSets'
      enableAutoScaling: true
      scaleSetPriority: systemPoolVmPriority
      count: 1
      minCount: 1
      maxCount: 3
      upgradeSettings: {
        maxSurge: '33%'
      }
    }

var userPoolBase = !empty(userPoolConfig)
  ? userPoolConfig
  : {
      vmSize: userPoolVmSize
      osType: 'Linux'
      osDiskType: userPoolVmDiskType
      osDiskSizeGB: int(userPoolVmDiskSizeGB)
      maxPods: nodePoolMaxPods
      type: 'VirtualMachineScaleSets'
      enableAutoScaling: true
      scaleSetPriority: userPoolVmPriority
      count: 1
      minCount: 0
      maxCount: 10
    }

var agentPoolProfiles = concat(
  [
    union(
      {
        name: 'npsystem'
        mode: 'System'
        nodeLabels: {
          'kompox.dev/node-pool': 'system'
        }
      },
      systemPoolBase
    )
  ],
  contains(userPoolVmZones, '1')
    ? [
        union(
          {
            name: 'npuser1'
            mode: 'User'
            availabilityZones: ['1']
            nodeLabels: {
              'kompox.dev/node-pool': 'user'
              'kompox.dev/node-zone': '1'
            }
          },
          userPoolBase
        )
      ]
    : [],
  contains(userPoolVmZones, '2')
    ? [
        union(
          {
            name: 'npuser2'
            mode: 'User'
            availabilityZones: ['2']
            nodeLabels: {
              'kompox.dev/node-pool': 'user'
              'kompox.dev/node-zone': '2'
            }
          },
          userPoolBase
        )
      ]
    : [],
  contains(userPoolVmZones, '3')
    ? [
        union(
          {
            name: 'npuser3'
            mode: 'User'
            availabilityZones: ['3']
            nodeLabels: {
              'kompox.dev/node-pool': 'user'
              'kompox.dev/node-zone': '3'
            }
          },
          userPoolBase
        )
      ]
    : []
)

resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2021-12-01-preview' existing = if (!empty(logAnalyticsName)) {
  name: logAnalyticsName
}

resource aks 'Microsoft.ContainerService/managedClusters@2025-05-01' = {
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
    agentPoolProfiles: agentPoolProfiles
    networkProfile: {
      loadBalancerSku: 'standard'
      networkPlugin: networkPlugin
      networkPluginMode: networkPlugin == 'azure' && !empty(networkPluginMode) ? networkPluginMode : null
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
        enabled: false
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
