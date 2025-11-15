package tasks

// indexOf is a test helper function to check if a string contains a substring
// Used across multiple test files for error message validation
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// containsString is a test helper to check if a string contains a substring
func containsString(s, substr string) bool {
	return indexOf(s, substr) >= 0
}
