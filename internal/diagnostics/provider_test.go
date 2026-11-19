package diagnostics

import (
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"strings"
	"testing"
)

func TestProviderAccountIsVerifiedSeparatelyFromTransport(t *testing.T) {
	expected := "work-account"
	for _, tc := range []struct{ greeting, code string }{
		{"Hi work-account! You've successfully authenticated, but GitHub does not provide shell access.", ""},
		{"Hi personal-account! You've successfully authenticated, but GitHub does not provide shell access.", "provider.account_mismatch"},
		{"", "provider.unverified"},
		{"An unrelated banner", "provider.unverified"},
	} {
		err := verifyGreeting([]byte(tc.greeting), &expected)
		if tc.code == "" {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err == nil || model.AsError(err).Code != tc.code {
			t.Fatalf("greeting code: %v", err)
		}
	}
	b := &greetingBuffer{}
	_, _ = b.Write([]byte(strings.Repeat("x", 10000)))
	if b.Len() != 4096 || !b.truncated {
		t.Fatal("unbounded greeting")
	}
}
