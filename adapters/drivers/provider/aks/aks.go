package aks

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
)

// driver implements the AKS provider driver.
type driver struct {
	TokenCredential     azcore.TokenCredential
	AzureSubscriptionId string
	AzureLocation       string
}

// ID returns the provider identifier.
func (d *driver) ID() string { return "aks" }

// init registers the AKS driver.
func init() {
	providerdrv.Register("aks", func(settings map[string]string) (providerdrv.Driver, error) {
		get := func(k string) string {
			if settings == nil {
				return ""
			}
			return strings.TrimSpace(settings[k])
		}

		subscriptionID := get("AZURE_SUBSCRIPTION_ID")
		location := get("AZURE_LOCATION")
		missing := make([]string, 0, 2)
		if subscriptionID == "" {
			missing = append(missing, "AZURE_SUBSCRIPTION_ID")
		}
		if location == "" {
			missing = append(missing, "AZURE_LOCATION")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("missing required AKS settings: %s", strings.Join(missing, ", "))
		}

		authMethod := get("AZURE_AUTH_METHOD")
		if authMethod == "" {
			return nil, fmt.Errorf("AZURE_AUTH_METHOD must be specified")
		}

		var cred azcore.TokenCredential
		var err error
		switch authMethod {
		case "client_secret":
			tenantID := get("AZURE_TENANT_ID")
			clientID := get("AZURE_CLIENT_ID")
			clientSecret := get("AZURE_CLIENT_SECRET")
			if tenantID == "" || clientID == "" || clientSecret == "" {
				return nil, fmt.Errorf("client_secret auth requires AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET")
			}
			cred, err = azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
		case "client_certificate":
			return nil, fmt.Errorf("client_certificate auth is not supported in this implementation (Go SDK requires x509.Certificate and crypto.PrivateKey parsing)")
		case "managed_identity":
			clientID := get("AZURE_CLIENT_ID")
			opts := &azidentity.ManagedIdentityCredentialOptions{}
			if clientID != "" {
				opts.ID = azidentity.ClientID(clientID)
			}
			cred, err = azidentity.NewManagedIdentityCredential(opts)
		case "workload_identity":
			tenantID := get("AZURE_TENANT_ID")
			clientID := get("AZURE_CLIENT_ID")
			tokenFile := get("AZURE_FEDERATED_TOKEN_FILE")
			if tenantID == "" || clientID == "" || tokenFile == "" {
				return nil, fmt.Errorf("workload_identity auth requires AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_FEDERATED_TOKEN_FILE")
			}
			cred, err = azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
				TenantID:      tenantID,
				ClientID:      clientID,
				TokenFilePath: tokenFile,
			})
		case "azure_cli":
			cred, err = azidentity.NewAzureCLICredential(nil)
		case "azure_developer_cli":
			cred, err = azidentity.NewAzureDeveloperCLICredential(nil)
		default:
			return nil, fmt.Errorf("unsupported AZURE_AUTH_METHOD: %s", authMethod)
		}
		if err != nil {
			return nil, fmt.Errorf("create Azure credential: %w", err)
		}

		return &driver{
			TokenCredential:     cred,
			AzureSubscriptionId: subscriptionID,
			AzureLocation:       location,
		}, nil
	})
}
