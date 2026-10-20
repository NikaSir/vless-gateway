package api

import (
	"crypto/sha256"
	"crypto/subtle"
)

func authorized(header, token string) bool {
	want := sha256.Sum256([]byte("Bearer " + token))
	got := sha256.Sum256([]byte(header))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}
