package model

// DNSRecordType represents provider-agnostic DNS record types.
type DNSRecordType string

const (
	DNSRecordTypeA     DNSRecordType = "A"
	DNSRecordTypeAAAA  DNSRecordType = "AAAA"
	DNSRecordTypeCNAME DNSRecordType = "CNAME"
	DNSRecordTypeTXT   DNSRecordType = "TXT"
	DNSRecordTypeMX    DNSRecordType = "MX"
	DNSRecordTypeNS    DNSRecordType = "NS"
	DNSRecordTypeSRV   DNSRecordType = "SRV"
	DNSRecordTypeCAA   DNSRecordType = "CAA"
)

// DNSRecordSet describes a single DNS record set identified by FQDN and type.
type DNSRecordSet struct {
	FQDN  string // Absolute FQDN. Trailing dot is optional.
	Type  DNSRecordType
	TTL   uint32   // TTL in seconds. Use provider default when zero.
	RData []string // Presentation-format RDATA. Empty slice indicates deletion.
}
