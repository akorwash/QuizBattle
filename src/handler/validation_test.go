package handler

import "testing"

func TestEmailLengthAndStructure(t *testing.T) {
	if IsEmailValid("a@b") {
		t.Fatal("email without a domain suffix should be rejected")
	}
	if IsEmailValid("a" + string(make([]byte, 255)) + "@example.com") {
		t.Fatal("oversized email should be rejected")
	}
	if !IsEmailValid("player@example.com") {
		t.Fatal("valid email was rejected")
	}
}

func TestUsernameStartsWithLetter(t *testing.T) {
	validator := ValidateUsername{}
	for _, invalid := range []string{"1player", "ab", "player name", "_player"} {
		if validator.Validate(invalid) {
			t.Fatalf("invalid username %q was accepted", invalid)
		}
	}
	if !validator.Validate("player.one") {
		t.Fatal("valid username was rejected")
	}
}

func TestPasswordBounds(t *testing.T) {
	validator := ValidatePassword{}
	if !validator.Validate("Strong#Pass1") {
		t.Fatal("valid password was rejected")
	}
	if validator.Validate("Aa#1short") {
		t.Fatal("short password was accepted")
	}
	if validator.Validate("A1#" + string(make([]byte, 73))) {
		t.Fatal("bcrypt-incompatible password was accepted")
	}
}
