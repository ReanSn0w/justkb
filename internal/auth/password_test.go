package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("password did not verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password verified")
	}
}

func TestPasswordMinimumLength(t *testing.T) {
	t.Parallel()
	if _, err := HashPassword("too-short"); err == nil {
		t.Fatal("expected short password error")
	}
}
