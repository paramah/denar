package token

import (
	"encoding/base64"
	"strings"
	"testing"

	"git.eiac.dev/tools/denar/internal/usecase"
)

var _ usecase.TokenGenerator = Generator{}

func TestNewToken(t *testing.T) {
	got, err := Generator{}.NewToken()
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("NewToken() = %q, not unpadded base64url: %v", got, err)
	}
	if len(raw) < 32 {
		t.Errorf("NewToken() carries %d bytes, want at least 32", len(raw))
	}
	if strings.ContainsAny(got, "+/=") {
		t.Errorf("NewToken() = %q, not URL-safe", got)
	}
}

func TestNewTokenDiffersEachCall(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		got, err := Generator{}.NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if seen[got] {
			t.Fatalf("NewToken() repeated %q", got)
		}
		seen[got] = true
	}
}
