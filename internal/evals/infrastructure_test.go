package evals_test

import (
	"testing"

	"deep-seeing/internal/evals"
)

func TestIsInfrastructureFailure(t *testing.T) {
	for _, message := range []string{
		"dial tcp: lookup gateway.example: i/o timeout",
		"Post https://gateway.example: net/http: TLS handshake timeout",
		"upstream returned status code: 503",
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
