param name string
param location string = resourceGroup().location
param tags object = {}

resource userIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2025-01-31-preview' = {
  name: name
  location: location
  tags: tags
}

output id string = userIdentity.id
output name string = userIdentity.name
output principalId string = userIdentity.properties.principalId
output clientId string = userIdentity.properties.clientId
