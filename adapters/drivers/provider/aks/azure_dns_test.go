package aks

import (
	"context"
	"testing"

	"github.com/kompox/kompox/domain/model"
)

func TestParseAzureDNSZoneID(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		wantName   string
		wantErr    bool
	}{
		{
			name:       "valid zone ID",
			resourceID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg/providers/Microsoft.Network/dnszones/example.com",
			wantName:   "example.com",
			wantErr:    false,
		},
		{
			name:       "valid zone ID with subdomain",
			resourceID: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/prod-rg/providers/Microsoft.Network/dnszones/app.example.com",
			wantName:   "app.example.com",
			wantErr:    false,
		},
		{
			name:       "invalid zone ID - too short",
			resourceID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg",
			wantErr:    true,
		},
		{
			name:       "invalid zone ID - wrong provider",
			resourceID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-rg/providers/Microsoft.Compute/dnszones/example.com",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAzureDNSZoneID(tt.resourceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAzureDNSZoneID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Name != tt.wantName {
					t.Errorf("parseAzureDNSZoneID() name = %v, want %v", got.Name, tt.wantName)
				}
				if got.ResourceID != tt.resourceID {
					t.Errorf("parseAzureDNSZoneID() id = %v, want %v", got.ResourceID, tt.resourceID)
				}
			}
		})
	}
}

func TestSelectDNSZone(t *testing.T) {
	ctx := context.Background()
	d := &driver{}

	zones := []*azureDNSZoneInfo{
		{
			ResourceID: "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/dnszones/example.com",
			Name:       "example.com",
		},
		{
			ResourceID: "/subscriptions/sub1/resourceGroups/rg2/providers/Microsoft.Network/dnszones/app.example.com",
			Name:       "app.example.com",
		},
		{
			ResourceID: "/subscriptions/sub1/resourceGroups/rg3/providers/Microsoft.Network/dnszones/other.net",
			Name:       "other.net",
		},
	}

	tests := []struct {
		name     string
		fqdn     string
		zoneHint string
		wantZone string
		wantErr  bool
	}{
		{
			name:     "exact match - APEX",
			fqdn:     "example.com",
			wantZone: "example.com",
			wantErr:  false,
		},
		{
			name:     "subdomain - longest match",
			fqdn:     "www.app.example.com",
			wantZone: "app.example.com",
			wantErr:  false,
		},
		{
			name:     "subdomain - parent zone",
			fqdn:     "api.example.com",
			wantZone: "example.com",
			wantErr:  false,
		},
		{
			name:    "no match",
			fqdn:    "notfound.org",
			wantErr: true,
		},
		{
			name:     "zone hint by name",
			fqdn:     "api.example.com",
			zoneHint: "app.example.com",
			wantZone: "app.example.com",
			wantErr:  false,
		},
		{
			name:     "zone hint by ID",
			fqdn:     "api.example.com",
			zoneHint: "/subscriptions/sub1/resourceGroups/rg3/providers/Microsoft.Network/dnszones/other.net",
			wantZone: "other.net",
			wantErr:  false,
		},
		{
			name:     "trailing dot in FQDN",
			fqdn:     "www.example.com.",
			wantZone: "example.com",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.selectDNSZone(ctx, tt.fqdn, zones, tt.zoneHint)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectDNSZone() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Name != tt.wantZone {
				t.Errorf("selectDNSZone() zone = %v, want %v", got.Name, tt.wantZone)
			}
		})
	}
}

func TestNormalizeDNSRecordSet(t *testing.T) {
	d := &driver{}

	tests := []struct {
		name    string
		rset    model.DNSRecordSet
		wantTTL uint32
		wantErr bool
	}{
		{
			name: "valid A record",
			rset: model.DNSRecordSet{
				FQDN:  "app.example.com",
				Type:  model.DNSRecordTypeA,
				TTL:   3600,
				RData: []string{"192.0.2.1"},
			},
			wantTTL: 3600,
			wantErr: false,
		},
		{
			name: "A record with TTL=0 normalized to 300",
			rset: model.DNSRecordSet{
				FQDN:  "app.example.com",
				Type:  model.DNSRecordTypeA,
				TTL:   0,
				RData: []string{"192.0.2.1"},
			},
			wantTTL: 300,
			wantErr: false,
		},
		{
			name: "valid CNAME record",
			rset: model.DNSRecordSet{
				FQDN:  "www.example.com",
				Type:  model.DNSRecordTypeCNAME,
				TTL:   600,
				RData: []string{"target.example.com"},
			},
			wantTTL: 600,
			wantErr: false,
		},
		{
			name: "CNAME with multiple RData - error",
			rset: model.DNSRecordSet{
				FQDN:  "www.example.com",
				Type:  model.DNSRecordTypeCNAME,
				TTL:   600,
				RData: []string{"target1.example.com", "target2.example.com"},
			},
			wantErr: true,
		},
		{
			name: "empty FQDN - error",
			rset: model.DNSRecordSet{
				FQDN:  "",
				Type:  model.DNSRecordTypeA,
				TTL:   300,
				RData: []string{"192.0.2.1"},
			},
			wantErr: true,
		},
		{
			name: "unsupported type - error",
			rset: model.DNSRecordSet{
				FQDN:  "app.example.com",
				Type:  model.DNSRecordTypeTXT,
				TTL:   300,
				RData: []string{"v=spf1"},
			},
			wantErr: true,
		},
		{
			name: "trailing dot removed",
			rset: model.DNSRecordSet{
				FQDN:  "app.example.com.",
				Type:  model.DNSRecordTypeA,
				TTL:   300,
				RData: []string{"192.0.2.1"},
			},
			wantTTL: 300,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			rset := tt.rset
			err := d.normalizeDNSRecordSet(&rset)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeDNSRecordSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if rset.TTL != tt.wantTTL {
					t.Errorf("normalizeDNSRecordSet() TTL = %v, want %v", rset.TTL, tt.wantTTL)
				}
				// Verify trailing dot is removed
				if rset.FQDN[len(rset.FQDN)-1] == '.' {
					t.Errorf("normalizeDNSRecordSet() FQDN still has trailing dot: %v", rset.FQDN)
				}
			}
		})
	}
}

func TestAzureDNSRecordSetName(t *testing.T) {
	d := &driver{}

	tests := []struct {
		name     string
		fqdn     string
		zoneName string
		want     string
	}{
		{
			name:     "APEX record",
			fqdn:     "example.com",
			zoneName: "example.com",
			want:     "@",
		},
		{
			name:     "subdomain",
			fqdn:     "www.example.com",
			zoneName: "example.com",
			want:     "www",
		},
		{
			name:     "nested subdomain",
			fqdn:     "api.app.example.com",
			zoneName: "example.com",
			want:     "api.app",
		},
		{
			name:     "APEX with trailing dot",
			fqdn:     "example.com.",
			zoneName: "example.com.",
			want:     "@",
		},
		{
			name:     "subdomain with trailing dot",
			fqdn:     "www.example.com.",
			zoneName: "example.com.",
			want:     "www",
		},
		{
			name:     "subzone APEX",
			fqdn:     "app.example.com",
			zoneName: "app.example.com",
			want:     "@",
		},
		{
			name:     "subdomain in subzone",
			fqdn:     "www.app.example.com",
			zoneName: "app.example.com",
			want:     "www",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.azureDNSRecordSetName(tt.fqdn, tt.zoneName)
			if got != tt.want {
				t.Errorf("azureDNSRecordSetName() = %v, want %v", got, tt.want)
			}
		})
	}
}
