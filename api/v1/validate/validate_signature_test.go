package validate

import "testing"

func TestValidateSignatureRejectsMalformedSignature(t *testing.T) {
	match, address, err := ValidateSignature(struct{}{}, 0, "x")
	if err != nil {
		t.Fatalf("malformed signature returned error: %v", err)
	}
	if match || address != "" {
		t.Fatal("malformed signature must not authenticate")
	}
}
