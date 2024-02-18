package utils

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const DEFAULT_REGION = "local"
const CUSTOM_EPOCH int64 = 1514764800000

func BoolToPtr(v bool) *bool {
	b := true
	return &b
}

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

// Region returns the current region of the GCP machine
func Region() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/zone", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		// can only send requests inside machine, otherwise we are in localhost
		return DEFAULT_REGION, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	response := string(body)

	var zone string
	parts := strings.Split(response, "/")
	if len(parts) >= 4 {
		zone = parts[3]
	} else {
		return "", fmt.Errorf("invalid response format: %s", response)
	}
	re := regexp.MustCompile(`-[a-z]$`)
	region := re.ReplaceAllString(zone, "")
	return region, nil
}
