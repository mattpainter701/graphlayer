package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/rs/zerolog"
)

var Logger = zerolog.Nop()

type MessageHelper struct{}

func BuildDeviceAssetNode(hostname, vendor, deviceType string, mgmtIP net.IP) map[string]interface{} {
	return map[string]interface{}{
		"hostname":    hostname,
		"vendor":      vendor,
		"device_type": deviceType,
		"mgmt_ip":     mgmtIP.String(),
	}
}

func ConvertToCIDR(ip net.IP, mask net.IPMask) string {
	return (&net.IPNet{IP: ip, Mask: mask}).String()
}

func EncodeToSha256(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

func RemoveDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func SliceFromMap(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func init() {
	_ = fmt.Sprintf
}
