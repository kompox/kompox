package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

const (
	// Default TTL for DNS records when not specified (5 minutes)
	defaultDNSRecordTTL = 300

	// Cluster settings key for DNS Zone Resource IDs
	settingAzureAKSDNSZoneResourceIDs = "AZURE_AKS_DNS_ZONE_RESOURCE_IDS"
)

// azureDNSZoneInfo represents parsed Azure DNS Zone resource information.
type azureDNSZoneInfo struct {
	ResourceID string // Full resource ID
	Name       string // Zone name (e.g., "example.com")
}

// parseAzureDNSZoneID parses an Azure DNS Zone resource ID using Azure SDK's parser.
// Expected format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/dnszones/{zone}
func parseAzureDNSZoneID(resourceID string) (*azureDNSZoneInfo, error) {
	rid, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return nil, fmt.Errorf("parse Azure DNS Zone resource ID: %w", err)
	}

	// Validate resource type
	if !strings.EqualFold(rid.ResourceType.Namespace, "Microsoft.Network") ||
		!strings.EqualFold(rid.ResourceType.Type, "dnszones") {
		return nil, fmt.Errorf("invalid resource type for DNS Zone: expected Microsoft.Network/dnszones, got %s/%s",
			rid.ResourceType.Namespace, rid.ResourceType.Type)
	}

	return &azureDNSZoneInfo{
		ResourceID: resourceID,
		Name:       rid.Name,
	}, nil
}

// collectDNSZoneIDs retrieves and parses DNS zone resource IDs from cluster settings.
func (d *driver) collectDNSZoneIDs(cluster *model.Cluster) ([]*azureDNSZoneInfo, error) {
	if cluster == nil || cluster.Settings == nil {
		return nil, nil
	}

	zoneIDsRaw := strings.TrimSpace(cluster.Settings[settingAzureAKSDNSZoneResourceIDs])
	if zoneIDsRaw == "" {
		return nil, nil
	}

	// Split by comma or space
	var zoneIDStrs []string
	for _, id := range strings.FieldsFunc(zoneIDsRaw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		id = strings.TrimSpace(id)
		if id != "" {
			zoneIDStrs = append(zoneIDStrs, id)
		}
	}

	zones := make([]*azureDNSZoneInfo, 0, len(zoneIDStrs))
	for _, id := range zoneIDStrs {
		info, err := parseAzureDNSZoneID(id)
		if err != nil {
			return nil, fmt.Errorf("parse DNS zone resource ID: %w", err)
		}
		zones = append(zones, info)
	}

	return zones, nil
}

// selectDNSZone selects the best matching DNS zone for the given FQDN.
// Priority: 1) ZoneHint (match by ID or name), 2) longest-match heuristic.
func (d *driver) selectDNSZone(ctx context.Context, fqdn string, zones []*azureDNSZoneInfo, zoneHint string) (*azureDNSZoneInfo, error) {
	log := logging.FromContext(ctx)

	if len(zones) == 0 {
		return nil, fmt.Errorf("no DNS zones configured in cluster.settings.%s", settingAzureAKSDNSZoneResourceIDs)
	}

	// Normalize FQDN: remove trailing dot
	fqdn = strings.TrimSuffix(fqdn, ".")

	// Step 1: Check ZoneHint (match by ID or name)
	if zoneHint != "" {
		for _, z := range zones {
			if z.ResourceID == zoneHint || z.Name == zoneHint {
				log.Debug(ctx, "DNS zone selected via hint", "fqdn", fqdn, "zone", z.Name, "zone_id", z.ResourceID)
				return z, nil
			}
		}
		log.Warn(ctx, "DNS zone hint did not match any configured zone", "hint", zoneHint)
	}

	// Step 2: Longest-match heuristic
	// Find the zone whose name is a suffix of the FQDN and has the longest match.
	var bestMatch *azureDNSZoneInfo
	bestMatchLen := 0
	for _, z := range zones {
		zoneName := strings.TrimSuffix(z.Name, ".")
		// Check if FQDN ends with zone name (exact match or subdomain)
		if fqdn == zoneName || strings.HasSuffix(fqdn, "."+zoneName) {
			if len(zoneName) > bestMatchLen {
				bestMatch = z
				bestMatchLen = len(zoneName)
			}
		}
	}

	if bestMatch != nil {
		log.Debug(ctx, "DNS zone selected via longest-match", "fqdn", fqdn, "zone", bestMatch.Name, "zone_id", bestMatch.ResourceID)
		return bestMatch, nil
	}

	return nil, fmt.Errorf("no matching DNS zone found for FQDN %s", fqdn)
}

// normalizeDNSRecordSet validates and normalizes the input record set.
// Returns error if validation fails.
func (d *driver) normalizeDNSRecordSet(rset *model.DNSRecordSet) error {
	// Validate FQDN
	if rset.FQDN == "" {
		return fmt.Errorf("FQDN is required")
	}

	// Normalize FQDN: remove trailing dot
	rset.FQDN = strings.TrimSuffix(rset.FQDN, ".")

	// Validate Type
	switch rset.Type {
	case model.DNSRecordTypeA, model.DNSRecordTypeAAAA, model.DNSRecordTypeCNAME:
		// Supported types
	default:
		return fmt.Errorf("unsupported DNS record type: %s", rset.Type)
	}

	// Validate CNAME: must have exactly 1 RData entry
	if rset.Type == model.DNSRecordTypeCNAME && len(rset.RData) > 1 {
		return fmt.Errorf("CNAME record must have exactly one RData entry, got %d", len(rset.RData))
	}

	// Normalize TTL: use default if zero
	if rset.TTL == 0 {
		rset.TTL = defaultDNSRecordTTL
	}

	return nil
}

// azureDNSRecordSetName converts FQDN to Azure DNS record set name (zone-relative).
// APEX records are represented as "@".
func (d *driver) azureDNSRecordSetName(fqdn string, zoneName string) string {
	fqdn = strings.TrimSuffix(fqdn, ".")
	zoneName = strings.TrimSuffix(zoneName, ".")

	// APEX case: FQDN matches zone name exactly
	if fqdn == zoneName {
		return "@"
	}

	// Subdomain case: remove zone suffix
	if strings.HasSuffix(fqdn, "."+zoneName) {
		relName := strings.TrimSuffix(fqdn, "."+zoneName)
		return relName
	}

	// Fallback: should not happen if zone selection is correct
	return fqdn
}

// upsertAzureDNSRecord creates or updates an Azure DNS record set.
func (d *driver) upsertAzureDNSRecord(ctx context.Context, zone *azureDNSZoneInfo, rset model.DNSRecordSet) error {
	log := logging.FromContext(ctx)

	// Parse zone resource ID to extract resource group and subscription
	rid, err := arm.ParseResourceID(zone.ResourceID)
	if err != nil {
		return fmt.Errorf("parse zone resource ID: %w", err)
	}

	client, err := armdns.NewRecordSetsClient(rid.SubscriptionID, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("create DNS record sets client: %w", err)
	}

	relName := d.azureDNSRecordSetName(rset.FQDN, zone.Name)
	recordType := armdns.RecordType(rset.Type)

	// Build record set properties based on type
	var properties armdns.RecordSetProperties
	properties.TTL = &[]int64{int64(rset.TTL)}[0]

	switch rset.Type {
	case model.DNSRecordTypeA:
		aRecords := make([]*armdns.ARecord, 0, len(rset.RData))
		for _, ip := range rset.RData {
			aRecords = append(aRecords, &armdns.ARecord{IPv4Address: &ip})
		}
		properties.ARecords = aRecords
	case model.DNSRecordTypeAAAA:
		aaaaRecords := make([]*armdns.AaaaRecord, 0, len(rset.RData))
		for _, ip := range rset.RData {
			aaaaRecords = append(aaaaRecords, &armdns.AaaaRecord{IPv6Address: &ip})
		}
		properties.AaaaRecords = aaaaRecords
	case model.DNSRecordTypeCNAME:
		if len(rset.RData) > 0 {
			properties.CnameRecord = &armdns.CnameRecord{Cname: &rset.RData[0]}
		}
	default:
		return fmt.Errorf("unsupported record type: %s", rset.Type)
	}

	recordSet := armdns.RecordSet{
		Properties: &properties,
	}

	log.Info(ctx, "upserting Azure DNS record",
		"zone_resource_id", zone.ResourceID,
		"record_name", relName,
		"type", rset.Type,
		"ttl", rset.TTL,
		"rdata", rset.RData,
	)

	_, err = client.CreateOrUpdate(ctx, rid.ResourceGroupName, zone.Name, relName, recordType, recordSet, nil)
	if err != nil {
		return fmt.Errorf("create/update DNS record: %w", err)
	}

	return nil
}

// deleteAzureDNSRecord deletes an Azure DNS record set.
func (d *driver) deleteAzureDNSRecord(ctx context.Context, zone *azureDNSZoneInfo, rset model.DNSRecordSet) error {
	log := logging.FromContext(ctx)

	// Parse zone resource ID to extract resource group and subscription
	rid, err := arm.ParseResourceID(zone.ResourceID)
	if err != nil {
		return fmt.Errorf("parse zone resource ID: %w", err)
	}

	client, err := armdns.NewRecordSetsClient(rid.SubscriptionID, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("create DNS record sets client: %w", err)
	}

	relName := d.azureDNSRecordSetName(rset.FQDN, zone.Name)
	recordType := armdns.RecordType(rset.Type)

	log.Info(ctx, "deleting Azure DNS record",
		"zone_resource_id", zone.ResourceID,
		"record_name", relName,
		"type", rset.Type,
	)

	_, err = client.Delete(ctx, rid.ResourceGroupName, zone.Name, relName, recordType, nil)
	if err != nil {
		return fmt.Errorf("delete DNS record: %w", err)
	}

	return nil
}

// ensureAzureDNSZoneRoles assigns DNS Zone Contributor role to the specified principal for all configured DNS zones.
// This is a best-effort operation: failures are logged as warnings and do not fail the overall operation.
func (d *driver) ensureAzureDNSZoneRoles(ctx context.Context, principalID string, zones []*azureDNSZoneInfo) {
	log := logging.FromContext(ctx)

	if principalID == "" {
		log.Warn(ctx, "ensureAzureDNSZoneRoles: principalID is empty, skipping DNS role assignments")
		return
	}

	if len(zones) == 0 {
		log.Debug(ctx, "ensureAzureDNSZoneRoles: no DNS zones configured, skipping DNS role assignments")
		return
	}

	roleDefID := d.azureRoleDefinitionID(roleDefIDDNSZoneContributor)

	for _, zone := range zones {
		logger := logging.FromContext(ctx).With("principalId", principalID, "scope", zone.ResourceID)
		if err := d.ensureAzureRole(ctx, zone.ResourceID, principalID, roleDefID); err != nil {
			logger.Info(ctx, "AKS:RoleDNS/efail", "err", err)
		} else {
			logger.Info(ctx, "AKS:RoleDNS/eok")
		}
	}
}
