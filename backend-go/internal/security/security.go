package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/pbkdf2"
)

// JWT — HS256, тот же формат payload, что в app/core/security.py: {sub, role, exp}.
type JWT struct{ secret []byte }

func NewJWT(secret string) *JWT { return &JWT{secret: []byte(secret)} }

func (j *JWT) Create(subject, role string, expireMinutes int) (string, error) {
	claims := jwt.MapClaims{
		"sub":  subject,
		"role": role,
		"exp":  time.Now().Add(time.Duration(expireMinutes) * time.Minute).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

func (j *JWT) Parse(token string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// VerifyPassword проверяет хеш passlib pbkdf2_sha256:
// формат $pbkdf2-sha256$<rounds>$<salt ab64>$<checksum ab64>.
func VerifyPassword(plain, hashed string) bool {
	parts := strings.Split(hashed, "$")
	if len(parts) != 5 || parts[1] != "pbkdf2-sha256" {
		return false
	}
	rounds, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}
	salt, err := ab64Decode(parts[3])
	if err != nil {
		return false
	}
	want, err := ab64Decode(parts[4])
	if err != nil {
		return false
	}
	got := pbkdf2.Key([]byte(plain), salt, rounds, len(want), sha256.New)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// HashPassword выдаёт хеш в формате passlib pbkdf2_sha256 (совместимо с Python-логином).
func HashPassword(plain string) string {
	const rounds = 29000
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	dk := pbkdf2.Key([]byte(plain), salt, rounds, 32, sha256.New)
	return fmt.Sprintf("$pbkdf2-sha256$%d$%s$%s", rounds, ab64Encode(salt), ab64Encode(dk))
}

// passlib "adapted base64": стандартный base64 без padding, '+' → '.'.
func ab64Decode(s string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(strings.ReplaceAll(s, ".", "+"))
}

func ab64Encode(b []byte) string {
	return strings.ReplaceAll(base64.RawStdEncoding.EncodeToString(b), "+", ".")
}

// RandToken — случайная строка (для пароля SMS-аккаунтов, он не используется для входа).
func RandToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
