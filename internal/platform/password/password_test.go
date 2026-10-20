package password

import (
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.PasswordHasher = Hasher{}

const secret = "correct horse battery staple"

// One Hash takes a few hundred milliseconds by design (64 MiB, 3 passes),
// so the tests share hashes and assert nothing about time.
func TestHashAndVerify(t *testing.T) {
	h := Hasher{}
	first, err := h.Hash(secret)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.Hash(secret)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(first, "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Errorf("Hash() = %q, want PHC argon2id with m=65536,t=3,p=1", first)
	}
	if strings.Contains(first, secret) {
		t.Errorf("Hash() = %q contains the password", first)
	}
	if first == second {
		t.Error("two hashes of one password are equal, want a random salt")
	}
	if !h.Verify(secret, first) || !h.Verify(secret, second) {
		t.Error("Verify() = false for the right password")
	}
	if h.Verify("wrong password", first) {
		t.Error("Verify() = true for a wrong password")
	}
	if h.Verify("", first) {
		t.Error("Verify() = true for an empty password")
	}
}

func TestVerifyMalformedHash(t *testing.T) {
	h := Hasher{}
	valid, err := h.Hash(secret)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, "$") // "", argon2id, v=19, params, salt, key
	with := func(i int, s string) string {
		p := append([]string(nil), parts...)
		p[i] = s
		return strings.Join(p, "$")
	}

	tests := map[string]string{
		"empty":             "",
		"plain password":    secret,
		"truncated":         valid[:len(valid)-10],
		"flipped key":       with(5, flip(parts[5])),
		"flipped salt":      with(4, flip(parts[4])),
		"other algorithm":   with(1, "argon2i"),
		"other version":     with(2, "v=16"),
		"missing parameter": with(3, "m=65536,t=3"),
		"zero passes":       with(3, "m=65536,t=0,p=1"),
		"zero threads":      with(3, "m=65536,t=3,p=0"),
		"zero memory":       with(3, "m=0,t=3,p=1"),
		"huge memory":       with(3, "m=4294967295,t=3,p=1"),
		"huge passes":       with(3, "m=65536,t=4294967295,p=1"),
		"not a number":      with(3, "m=lots,t=3,p=1"),
		"bad base64 salt":   with(4, "!!!"),
		"bad base64 key":    with(5, "!!!"),
		"empty salt":        with(4, ""),
		"empty key":         with(5, ""),
		"extra field":       valid + "$extra",
	}
	for name, hash := range tests {
		t.Run(name, func(t *testing.T) {
			if h.Verify(secret, hash) {
				t.Errorf("Verify(%q) = true", hash)
			}
		})
	}
}

// flip changes the first character of a base64 string to another valid one.
func flip(s string) string {
	if s[0] == 'A' {
		return "B" + s[1:]
	}
	return "A" + s[1:]
}
