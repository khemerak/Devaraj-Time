package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func Digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func Hash(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	key := argon2.IDKey([]byte(password), salt, 2, 64*1024, 2, 32)
	return fmt.Sprintf("argon2id$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}
func Verify(password, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 3 || p[0] != "argon2id" {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(p[1])
	if e != nil || len(salt) != 16 {
		return false
	}
	want, e := base64.RawStdEncoding.DecodeString(p[2])
	if e != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 2, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
