package security

import "testing"

// Вектор из passlib (Python): ctx.hash("secret123").
const passlibHash = "$pbkdf2-sha256$29000$0/r/P4dQao1R6v2/19obQw$GKV1WVF6LcmdTNzAX.IXAT62ntIGuVSD9r6AsLxg2uE"

// JWT из PyJWT, подписан "test-secret-key".
const pyJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJndXJ1QG1hbmliYW5kaGEucHJlbWEuc3UiLCJyb2xlIjoiZ3VydSIsImV4cCI6MTc4NDI4ODU4MX0.rcjHTnvsLVvxQdAonepS99Y_DQgLjlqwgmCOtVX-dO4"

func TestVerifyPasslibHash(t *testing.T) {
	if !VerifyPassword("secret123", passlibHash) {
		t.Fatal("passlib pbkdf2_sha256 hash should verify with correct password")
	}
	if VerifyPassword("wrong", passlibHash) {
		t.Fatal("must reject wrong password")
	}
}

func TestHashRoundTrip(t *testing.T) {
	h := HashPassword("hunter2")
	if !VerifyPassword("hunter2", h) {
		t.Fatal("round-trip verify failed")
	}
	if VerifyPassword("nope", h) {
		t.Fatal("must reject wrong password on round-trip")
	}
}

func TestParsePyJWT(t *testing.T) {
	j := NewJWT("test-secret-key")
	claims, err := j.Parse(pyJWT)
	if err != nil {
		t.Fatalf("parse python JWT: %v", err)
	}
	if claims["sub"] != "guru@manibandha.prema.su" {
		t.Fatalf("sub = %v", claims["sub"])
	}
	if claims["role"] != "guru" {
		t.Fatalf("role = %v", claims["role"])
	}
	// неверный секрет — отказ
	if _, err := NewJWT("other").Parse(pyJWT); err == nil {
		t.Fatal("must reject token signed with a different secret")
	}
}
