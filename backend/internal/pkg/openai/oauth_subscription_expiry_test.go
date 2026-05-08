package openai

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Helper to create a JWT with custom payload (no signature validation)
func makeTestJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)
	enc := func(b []byte) string {
		return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
	}
	return enc(headerJSON) + "." + enc(payloadJSON) + ".signature"
}

func TestExtractSubscriptionExpiresFromIDToken_EmptyToken(t *testing.T) {
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken(""))
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken("   "))
}

func TestExtractSubscriptionExpiresFromIDToken_InvalidFormat(t *testing.T) {
	// Invalid JWT formats - should not panic
	testCases := []string{
		"not-a-jwt",
		"only.two",
		"three..parts",
		"....",
	}
	for _, tc := range testCases {
		require.Empty(t, ExtractSubscriptionExpiresFromIDToken(tc), "should handle %q without panic", tc)
	}
}

func TestExtractSubscriptionExpiresFromIDToken_InvalidBase64(t *testing.T) {
	// Invalid base64 - should not panic
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken("a.!@#$.c"))
}

func TestExtractSubscriptionExpiresFromIDToken_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON
	invalidJSON := "a." + strings.TrimRight(base64.URLEncoding.EncodeToString([]byte("not json")), "=") + ".c"
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken(invalidJSON))
}

func TestExtractSubscriptionExpiresFromIDToken_NoOpenAIAuthClaim(t *testing.T) {
	// Valid JWT without OpenAI auth claim
	payload := map[string]any{
		"sub":   "user123",
		"email": "test@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := makeTestJWT(t, payload)
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken(token))
}

func TestExtractSubscriptionExpiresFromIDToken_NoExpiryInClaim(t *testing.T) {
	// Valid JWT with OpenAI auth claim but no subscription_expires_at
	payload := map[string]any{
		"sub": "user123",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acc-123",
			"chatgpt_plan_type":  "plus",
		},
	}
	token := makeTestJWT(t, payload)
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken(token))
}

func TestExtractSubscriptionExpiresFromIDToken_NullExpiry(t *testing.T) {
	payload := map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"subscription_expires_at": nil,
		},
	}
	token := makeTestJWT(t, payload)
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken(token))
}

func TestExtractSubscriptionExpiresFromIDToken_EmptyString(t *testing.T) {
	payload := map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"subscription_expires_at": "",
		},
	}
	token := makeTestJWT(t, payload)
	require.Empty(t, ExtractSubscriptionExpiresFromIDToken(token))
}

func TestExtractSubscriptionExpiresFromIDToken_ValidRFC3339String(t *testing.T) {
	expiry := "2026-05-02T20:32:12+00:00"
	payload := map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"subscription_expires_at": expiry,
		},
	}
	token := makeTestJWT(t, payload)
	result := ExtractSubscriptionExpiresFromIDToken(token)
	require.Equal(t, expiry, result)
}

func TestExtractSubscriptionExpiresFromIDToken_ValidUnixTimestampNumber(t *testing.T) {
	// Unix timestamp for 2026-05-02T20:32:12Z
	ts := int64(1777777132)
	payload := map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"subscription_expires_at": ts,
		},
	}
	token := makeTestJWT(t, payload)
	result := ExtractSubscriptionExpiresFromIDToken(token)
	require.NotEmpty(t, result)
	// Should parse as valid time
	parsed, err := time.Parse(time.RFC3339, result)
	require.NoError(t, err)
	require.Equal(t, ts, parsed.Unix())
}

func TestExtractSubscriptionExpiresFromIDToken_ValidUnixTimestampString(t *testing.T) {
	// Unix timestamp as string
	payload := map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"subscription_expires_at": "1777777132",
		},
	}
	token := makeTestJWT(t, payload)
	result := ExtractSubscriptionExpiresFromIDToken(token)
	require.NotEmpty(t, result)
	parsed, err := time.Parse(time.RFC3339, result)
	require.NoError(t, err)
	require.Equal(t, int64(1777777132), parsed.Unix())
}

func TestExtractSubscriptionExpiresFromIDToken_InvalidTimestamp(t *testing.T) {
	// Invalid values should return empty
	testCases := []any{
		int64(-1),
		int64(0),
		"not-a-date",
		"2026-13-01T00:00:00Z", // invalid month
		"abc",
	}
	for _, tc := range testCases {
		payload := map[string]any{
			"https://api.openai.com/auth": map[string]any{
				"subscription_expires_at": tc,
			},
		}
		token := makeTestJWT(t, payload)
		require.Empty(t, ExtractSubscriptionExpiresFromIDToken(token), "should handle %v without panic", tc)
	}
}

func TestIDTokenClaims_ExtractSubscriptionExpiresAt_NilReceiver(t *testing.T) {
	var claims *IDTokenClaims
	require.Empty(t, claims.ExtractSubscriptionExpiresAt())
}

func TestIDTokenClaims_ExtractSubscriptionExpiresAt_NilOpenAIAuth(t *testing.T) {
	claims := &IDTokenClaims{
		Sub: "user123",
	}
	require.Empty(t, claims.ExtractSubscriptionExpiresAt())
}

func TestIDTokenClaims_ExtractSubscriptionExpiresAt_Valid(t *testing.T) {
	expiry := "2026-12-31T23:59:59Z"
	claims := &IDTokenClaims{
		OpenAIAuth: &OpenAIAuthClaims{
			SubscriptionExpiresAtRaw: expiry,
		},
	}
	require.Equal(t, expiry, claims.ExtractSubscriptionExpiresAt())
}
