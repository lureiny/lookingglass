package ws

import "strings"

// maskIPv6Address masks IPv6 addresses, keeping only first and last segments
// Examples:
//   2001:0db8:85a3:0000:0000:8a2e:0370:7334 -> 2001:****:****:****:****:****:****:7334
//   2001:0db8::8a2e:0370:7334 -> 2001:****:****:****:****:****:****:7334
//   fe80::1 -> fe80:****:****:****:****:****:****:1
func maskIPv6Address(ip string) string {
	if ip == "" {
		return ip
	}

	// Expand :: to full format first
	expandedIP := expandIPv6(ip)
	if expandedIP == "" {
		// Invalid IPv6, return as-is
		return ip
	}

	parts := strings.Split(expandedIP, ":")
	if len(parts) != 8 {
		// Not a valid expanded IPv6, return as-is
		return ip
	}

	// Keep first and last segment, mask the middle 6 segments
	parts[1] = "****"
	parts[2] = "****"
	parts[3] = "****"
	parts[4] = "****"
	parts[5] = "****"
	parts[6] = "****"

	return strings.Join(parts, ":")
}

// expandIPv6 expands IPv6 address with :: notation to full 8-segment format
// Example: 2001:0db8::8a2e:0370:7334 -> 2001:0db8:0000:0000:0000:8a2e:0370:7334
func expandIPv6(ip string) string {
	if ip == "" {
		return ""
	}

	// Handle special case: starts or ends with ::
	if strings.HasPrefix(ip, "::") {
		ip = "0" + ip
	}
	if strings.HasSuffix(ip, "::") {
		ip = ip + "0"
	}

	// If no :: compression, just validate and return
	if !strings.Contains(ip, "::") {
		parts := strings.Split(ip, ":")
		if len(parts) != 8 {
			return ""
		}
		// Pad each segment to 4 digits
		for i, part := range parts {
			if len(part) == 0 || len(part) > 4 {
				return ""
			}
			parts[i] = padHex(part)
		}
		return strings.Join(parts, ":")
	}

	// Split by ::
	halves := strings.Split(ip, "::")
	if len(halves) != 2 {
		// Multiple :: or invalid format
		return ""
	}

	leftParts := []string{}
	if halves[0] != "" {
		leftParts = strings.Split(halves[0], ":")
	}

	rightParts := []string{}
	if halves[1] != "" {
		rightParts = strings.Split(halves[1], ":")
	}

	// Calculate how many zero segments to insert
	missingCount := 8 - len(leftParts) - len(rightParts)
	if missingCount < 1 {
		return ""
	}

	// Build full address
	result := make([]string, 8)
	idx := 0

	// Add left parts
	for _, part := range leftParts {
		if len(part) == 0 || len(part) > 4 {
			return ""
		}
		result[idx] = padHex(part)
		idx++
	}

	// Add zero segments
	for i := 0; i < missingCount; i++ {
		result[idx] = "0000"
		idx++
	}

	// Add right parts
	for _, part := range rightParts {
		if len(part) == 0 || len(part) > 4 {
			return ""
		}
		result[idx] = padHex(part)
		idx++
	}

	return strings.Join(result, ":")
}

// padHex pads a hex string to 4 digits with leading zeros
func padHex(s string) string {
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}
