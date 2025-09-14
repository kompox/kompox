#!/bin/bash

LOCATION=${1-japaneast}

{
	echo -e "Name\tvCPU\tMemGB\tTempMB\tNVMeMB\tCacheBytes\tZones\tRestrictions" 
	az vm list-skus \
		--location $LOCATION \
		--resource-type virtualMachines \
		--output tsv \
		--query "[?resourceType=='virtualMachines'] | [].{
				Name: name,
				vCPU: to_number(capabilities[?name=='vCPUs'].value | [0]),
				MemGB: to_number(capabilities[?name=='MemoryGB'].value | [0]),
				TempMB: to_number(capabilities[?name=='MaxResourceVolumeMB'].value | [0]),
				NVMeMB: to_number(capabilities[?name=='NvmeDiskSizeInMiB'].value | [0]),
				CacheBytes: to_number(capabilities[?name=='CachedDiskBytes'].value | [0]),
				Zones: join(',', sort(locationInfo[].zones[])),
				Restrictions: join(' ',restrictions[?type=='Zone'].join(':',[reasonCode,restrictionInfo.locations[0],join(',',restrictionInfo.zones)]))
			}"
} | column -t -s $'\t'
