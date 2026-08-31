package evals_test

import (
	"testing"

	"deep-seeing/internal/evals"
)

func TestIsInfrastructureFailure(t *testing.T) {
	for _, message := range []string{
		"dial tcp: lookup gateway.example: i/o timeout",
		"read: operation timed out",
		"Client.Timeout exceeded while awaiting headers: context deadline exceeded",
		"read: can't assign requested address",
		"Post https://gateway.example: net/http: TLS handshake timeout",
		"upstream returned status code: 503",
		"chat status 503: upstream unavailable",
		"stream: unexpected EOF",
	} {
		if !evals.IsInfrastructureFailure(message) {
			t.Fatalf("%q should be infrastructure", message)
		}
	}
	for _, message := range []string{
		"",
		"agent reached max step count",
		"report_recall_evidence rejected invalid id",
		"answer empty",
	} {
		if evals.IsInfrastructureFailure(message) {
			t.Fatalf("%q should remain a behavior failure", message)
		}
	}
}
