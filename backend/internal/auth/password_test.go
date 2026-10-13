package auth

import "testing"

func TestPasswords(t *testing.T) {
	h := Hash("a sufficiently long password")
	if !Verify("a sufficiently long password", h) || Verify("wrong", h) || Verify("x", "garbage") {
		t.Fatal("password verification failed")
	}
	if h == Hash("a sufficiently long password") {
		t.Fatal("salt reused")
	}
}
func TestTokens(t *testing.T) {
	a, b := Token(), Token()
	if len(a) != 43 || a == b || Digest(a) == a {
		t.Fatal("insecure tokens")
	}
}
