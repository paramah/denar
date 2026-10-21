package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func key(b byte) []byte { return bytes.Repeat([]byte{b}, KeySize) }

func TestSealOpen(t *testing.T) {
	box, err := NewBox(key(1))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal([]byte(`{"token":"s3cret"}`), []byte("ch1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("s3cret")) {
		t.Fatal("the plaintext is visible")
	}
	again, _ := box.Seal([]byte(`{"token":"s3cret"}`), []byte("ch1"))
	if bytes.Equal(sealed, again) {
		t.Error("two seals are equal: the nonce repeats")
	}
	plain, err := box.Open(sealed, []byte("ch1"))
	if err != nil || string(plain) != `{"token":"s3cret"}` {
		t.Fatalf("open = %q, %v", plain, err)
	}
	if _, err := box.Open(sealed, []byte("ch2")); !errors.Is(err, ErrOpen) {
		t.Errorf("other aad = %v", err)
	}
	other, _ := NewBox(key(2))
	if _, err := other.Open(sealed, []byte("ch1")); !errors.Is(err, ErrOpen) {
		t.Errorf("other key = %v", err)
	}
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	if _, err := box.Open(tampered, []byte("ch1")); !errors.Is(err, ErrOpen) {
		t.Errorf("tampered = %v", err)
	}
	for _, bad := range [][]byte{nil, {version}, append([]byte{9}, sealed[1:]...)} {
		if _, err := box.Open(bad, []byte("ch1")); !errors.Is(err, ErrOpen) {
			t.Errorf("open(%x) = %v", bad, err)
		}
	}
}

func TestParseKey(t *testing.T) {
	k := key(7)
	for _, s := range []string{base64.StdEncoding.EncodeToString(k), base64.RawURLEncoding.EncodeToString(k), " " + base64.StdEncoding.EncodeToString(k) + "\n"} {
		got, err := ParseKey(s)
		if err != nil || !bytes.Equal(got, k) {
			t.Errorf("ParseKey(%q) = %x, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := ParseKey(s); err == nil {
			t.Errorf("ParseKey(%q) passed", s)
		}
	}
	if _, err := NewBox([]byte("short")); err == nil {
		t.Error("short key accepted")
	}
}
