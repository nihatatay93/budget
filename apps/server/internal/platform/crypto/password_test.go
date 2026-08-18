package crypto

import (
	"strings"
	"testing"
)

func TestPasswordHasherRoundTrip(t *testing.T) {
	hasher := PasswordHasher{}
	encoded, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("Hash() = %q, want documented Argon2id parameters", encoded)
	}
	valid, err := hasher.Verify("correct horse battery staple", encoded)
	if err != nil || !valid {
		t.Fatalf("Verify(correct) = %v, %v", valid, err)
	}
	valid, err = hasher.Verify("incorrect password", encoded)
	if err != nil || valid {
		t.Fatalf("Verify(incorrect) = %v, %v", valid, err)
	}
}

func TestPasswordHasherRejectsUnsupportedParameters(t *testing.T) {
	hasher := PasswordHasher{}
	_, err := hasher.Verify(
		"password",
		"$argon2id$v=19$m=1024,t=1,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	)
	if err == nil {
		t.Fatal("Verify() error = nil, want unsupported parameters error")
	}
}
