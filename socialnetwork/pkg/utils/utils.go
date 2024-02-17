package utils

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const DEFAULT_REGION = "local"

func BoolToPtr(v bool) *bool {
	b := true
	return &b
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
