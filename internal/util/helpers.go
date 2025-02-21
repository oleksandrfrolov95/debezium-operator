package util

// ConfigsEqual compares two configuration maps for equality.
func ConfigsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bVal, exists := b[k]; !exists || bVal != v {
			return false
		}
	}
	return true
}
