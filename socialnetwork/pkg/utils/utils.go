package utils

import (
	"strconv"
	"strings"
)

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
