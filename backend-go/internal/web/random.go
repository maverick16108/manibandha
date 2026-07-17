package web

import (
	"crypto/rand"
	"encoding/binary"
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
