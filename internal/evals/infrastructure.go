package evals

import "strings"

// IsInfrastructureFailure recognizes transport and gateway failures that do
// not provide a valid Agent behavior sample. Product errors such as step-limit
// exhaustion are deliberately excluded and remain behavior failures.
func IsInfrastructureFailure(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	for _, marker := range []string{
		"dial tcp",
		"no such host",
		"i/o timeout",
		"client.timeout",
		"tls handshake timeout",
		"connection reset",
		"connection refused",
		"unexpected eof",
		"server misbehaving",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"status code: 502",
		"status code: 503",
		"status code: 504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
