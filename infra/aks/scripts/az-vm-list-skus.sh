#!/bin/bash

LOCATION=${1-japaneast}

# Check for jq
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed." >&2
    exit 1
fi

# Temporary files
SKU_FILE=$(mktemp)
RAW_PRICE_FILE=$(mktemp)

# Cleanup on exit
trap "rm -f $SKU_FILE $RAW_PRICE_FILE" EXIT

# 1. Get VM SKUs
echo "Fetching VM SKUs for $LOCATION..." >&2
az vm list-skus \
	--location "$LOCATION" \
	--resource-type virtualMachines \
	--output tsv \
	--query "[?resourceType=='virtualMachines'] | [].{
			Name: name,
			vCPU: to_number(capabilities[?name=='vCPUs'].value | [0]),
			MemGB: to_number(capabilities[?name=='MemoryGB'].value | [0]),
			TempMB: to_number(capabilities[?name=='MaxResourceVolumeMB'].value | [0]),
			NVMeMB: to_number(capabilities[?name=='NvmeDiskSizeInMiB'].value | [0]),
			DataDisks: to_number(capabilities[?name=='MaxDataDiskCount'].value | [0]),
			LowPriority: to_string(capabilities[?name=='LowPriorityCapable'].value | [0]),
			Zones: join(',', sort(locationInfo[].zones[])),
			Restrictions: join(' ',restrictions[?type=='Zone'].join(':',[reasonCode,restrictionInfo.locations[0],join(',',restrictionInfo.zones)]))
		}" > "$SKU_FILE"

# 2. Get Prices from Azure Retail Prices API
echo "Fetching prices from Azure Retail Prices API..." >&2

# Filter: Virtual Machines, Region, Consumption, USD
API_URL="https://prices.azure.com/api/retail/prices?\$filter=serviceName%20eq%20'Virtual%20Machines'%20and%20armRegionName%20eq%20'$LOCATION'%20and%20type%20eq%20'Consumption'%20and%20currencyCode%20eq%20'USD'"

NEXT_LINK="$API_URL"
PAGE=1

while [ "$NEXT_LINK" != "null" ] && [ -n "$NEXT_LINK" ]; do
	echo "Fetching prices page $PAGE..." >&2
	RESPONSE=$(curl -s "$NEXT_LINK")

	# Extract armSkuName, retailPrice, and determine if it's Spot/Low Priority
	# Filter out Windows products
	echo "$RESPONSE" | jq -r '.Items[] | select(.productName | contains("Windows") | not) |
		[
			.armSkuName,
			.retailPrice,
			(if (.meterName | test("Spot|Low Priority")) then "Spot" else "Regular" end)
		] | @tsv' >> "$RAW_PRICE_FILE"

	NEXT_LINK=$(echo "$RESPONSE" | jq -r '.NextPageLink')
	((PAGE++))
done

# 3. Merge and Output
echo "Merging data..." >&2

SUB_INFO=$(az account show --query '{name:name, id:id}' -o tsv)
SUB_NAME=$(echo "$SUB_INFO" | cut -f1)
SUB_ID=$(echo "$SUB_INFO" | cut -f2)
CURRENT_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Subscription: $SUB_NAME ($SUB_ID)"
echo "Location: $LOCATION"
echo "Date: $CURRENT_DATE"
echo ""

{
	echo -e "Name\tvCPU\tMemGB\tTempMB\tNVMeMB\tDataDisks\tSpot\tPrice/Hour\tPrice/Month\tSpot/Hour\tZones\tRestrictions"

	awk 'BEGIN {FS="\t"; OFS="\t"}
		# Process Price File (FNR==NR)
		FNR==NR {
			sku = $1
			price = $2 + 0
			type = $3

			if (type == "Regular") {
				if (regular[sku] == "" || price < regular[sku]) {
					regular[sku] = price
				}
			} else {
				if (spot[sku] == "" || price < spot[sku]) {
					spot[sku] = price
				}
			}
			next
		}

		# Process SKU File
		{
			sku = $1

			# Regular Price
			if (regular[sku] != "") {
				p_hour = sprintf("%10.4f", regular[sku])
				p_month = sprintf("%11.2f", regular[sku] * 730)
			} else {
				p_hour = sprintf("%10s", "N/A")
				p_month = sprintf("%11s", "N/A")
			}

			# Spot Price
			if (spot[sku] != "") {
				s_hour = sprintf("%10.4f", spot[sku])
			} else {
				s_hour = sprintf("%10s", "-")
			}

			print $1, $2, $3, $4, $5, $6, $7, p_hour, p_month, s_hour, $8, $9
		}' "$RAW_PRICE_FILE" "$SKU_FILE"
} | column -t -s $'\t'
