package web

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
)

// randomInt возвращает криптослучайное число в [0, n).
func randomInt(n int) int {
	if n <= 0 {
		return 0
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	return int(binary.BigEndian.Uint32(b[:]) % uint32(n))
}

// randHex — 32 hex-символа (16 случайных байт). Достаточно для любых [:N] срезов.
func randHex() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
