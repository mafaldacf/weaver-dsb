package utils

import (
	"bytes"
	"net"
	"os"
	"strconv"
	"strings"
)

const DEFAULT_REGION = "local"
const CUSTOM_EPOCH int64 = 1514764800000

// from original deathstarbench code
func HashMacAddressPid(mac string) string {
	var hash uint16 = 0
	macPid := mac + strconv.Itoa(os.Getpid())
	for i := 0; i < len(macPid); i++ {
		hash += uint16(macPid[i] << (i & 1) * 8)
	}

	hashStr := strconv.FormatUint(uint64(hash), 10)
	// truncate hash
	if len(hashStr) > 3 {
		hashStr = hashStr[:3]
	} else if len(hashStr) < 3 {
		hashStr = strings.Repeat("0", 3-len(hashStr)) + hashStr
	}
	return hashStr
}

// inspired from
// https://gitlab.mpi-sws.org/cld/blueprint/systems/blueprint-dsb-socialnetwork/-/blob/main/input_v1/input_go/services/UniqueIdService.go
// https://gist.github.com/liasica/f4db81e8138b5b0d7978e4e55933914a
func GetMachineID() string {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, i := range interfaces {
			if i.Flags&net.FlagUp != 0 && !bytes.Equal(i.HardwareAddr, nil) {
				// Skip locally administered addresses
				if i.HardwareAddr[0]&2 == 2 {
					continue
				}

				mac := i.HardwareAddr
				return HashMacAddressPid(mac.String())
			}
		}
	}
	return "0"
}

func GenUniqueID(machineID string, timestamp int64, counter int64) (int64, error) {
	timestampHex := strconv.FormatInt(timestamp, 16)

	if len(timestampHex) > 10 {
		timestampHex = timestampHex[:10]
	} else if len(timestampHex) < 10 {
		timestampHex = strings.Repeat("0", 10-len(timestampHex)) + timestampHex
	}

	counterHex := strconv.FormatInt(counter, 16)
	if len(counterHex) > 3 {
		counterHex = counterHex[:3]
	} else if len(counterHex) < 3 {
		counterHex = strings.Repeat("0", 3-len(counterHex)) + counterHex
	}

	uniqueIDStr := machineID + timestampHex + counterHex
	uniqueID, err := strconv.ParseInt(uniqueIDStr, 16, 64)
	if err != nil {
		return 0, err
	}
	uniqueID = uniqueID & 0x7FFFFFFFFFFFFFFF
	return uniqueID, nil
}
