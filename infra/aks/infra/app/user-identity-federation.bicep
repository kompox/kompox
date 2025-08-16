param name string
param userIdentityName string
param issuerUrl string
param subject string
param audience string

resource userIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2025-01-31-preview' existing = {
  name: userIdentityName
}

resource federatedIdentityCredential 'Microsoft.ManagedIdentity/userAssignedIdentities/federatedIdentityCredentials@2025-01-31-preview' = {
  parent: userIdentity
  name: name
  properties: {
    issuer: issuerUrl
    subject: subject
    audiences: [
      audience
    ]
  }
}
