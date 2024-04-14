package utils

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)
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
