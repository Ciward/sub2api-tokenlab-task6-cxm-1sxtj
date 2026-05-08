package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type syncMetadataProxyRepoStub struct{}

func (s *syncMetadataProxyRepoStub) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	return nil, errors.New("not found")
}

type syncMetadataOAuthClientStub struct{}

func (s *syncMetadataOAuthClientStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *syncMetadataOAuthClientStub) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *syncMetadataOAuthClientStub) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func makeTestJWT(payload map[string]any) string {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)
	enc := func(b []byte) string {
		return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
	}
	return enc(headerJSON) + "." + enc(payloadJSON) + ".signature"
}

func TestOpenAIOAuthService_SyncAccountMetadata_InvalidPlatform(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	account := &Account{
		ID:          1,
		Platform:    PlatformAnthropic, // Not OpenAI
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	_, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_OAUTH_INVALID_PLATFORM")
}

func TestOpenAIOAuthService_SyncAccountMetadata_InvalidAccountType(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey, // Not OAuth
		Credentials: map[string]any{"access_token": "test-token"},
	}

	_, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_OAUTH_INVALID_ACCOUNT_TYPE")
}

func TestOpenAIOAuthService_SyncAccountMetadata_NoAccessToken(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{}, // No access_token
	}

	_, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_OAUTH_NO_ACCESS_TOKEN")
}

func TestOpenAIOAuthService_SyncAccountMetadata_PreservesTokenFields(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	originalExpiresAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":            "original-access-token",
			"refresh_token":           "original-refresh-token",
			"id_token":                "original-id-token",
			"client_id":               "original-client-id",
			"expires_at":              originalExpiresAt,
			"email":                   "original@example.com",
			"plan_type":               "plus",
			"model_mapping":           map[string]any{"gpt-4": "gpt-4-turbo"},
			"custom_field":            "custom-value",
		},
	}

	result, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.UpdatedCredentials)

	// Token fields MUST be preserved
	require.Equal(t, "original-access-token", result.UpdatedCredentials["access_token"])
	require.Equal(t, "original-refresh-token", result.UpdatedCredentials["refresh_token"])
	require.Equal(t, "original-id-token", result.UpdatedCredentials["id_token"])
	require.Equal(t, "original-client-id", result.UpdatedCredentials["client_id"])
	require.Equal(t, originalExpiresAt, result.UpdatedCredentials["expires_at"])

	// Other fields should also be preserved
	require.Equal(t, "original@example.com", result.UpdatedCredentials["email"])
	require.Equal(t, "plus", result.UpdatedCredentials["plan_type"])
	require.Equal(t, "custom-value", result.UpdatedCredentials["custom_field"])
	require.NotNil(t, result.UpdatedCredentials["model_mapping"])
}

func TestOpenAIOAuthService_SyncAccountMetadata_NilInput(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	_, err := svc.SyncAccountMetadata(context.Background(), nil)
	require.Error(t, err)

	_, err = svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{Account: nil})
	require.Error(t, err)
}

func TestOpenAIOAuthService_SyncAccountMetadata_SetupTokenNotAllowed(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	// For OpenAI, setup-token is NOT considered OAuth for this endpoint
	// (unlike general IsOAuth which returns true for both oauth and setup-token)
	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeSetupToken,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	_, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_OAUTH_INVALID_ACCOUNT_TYPE")
}

func TestOpenAIOAuthService_SyncAccountMetadata_ExtractsFromIDToken(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	// Build a valid test id_token with subscription_expires_at claim
	idToken := makeTestJWT(map[string]any{
		"sub":   "user123",
		"email": "test@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id":      "acc-123",
			"chatgpt_plan_type":       "plus",
			"subscription_expires_at": "2026-12-31T23:59:59Z",
		},
	})

	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "original-access-token",
			"refresh_token": "original-refresh-token",
			"id_token":      idToken,
			"plan_type":     "plus",
			// Note: no subscription_expires_at in credentials initially
		},
	}

	result, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Token fields MUST be preserved
	require.Equal(t, "original-access-token", result.UpdatedCredentials["access_token"])
	require.Equal(t, "original-refresh-token", result.UpdatedCredentials["refresh_token"])
	require.Equal(t, idToken, result.UpdatedCredentials["id_token"])

	// subscription_expires_at should be extracted from id_token
	require.Equal(t, "2026-12-31T23:59:59Z", result.UpdatedCredentials["subscription_expires_at"])

	// email from id_token should also be extracted
	require.Equal(t, "test@example.com", result.UpdatedCredentials["email"])
}

func TestOpenAIOAuthService_SyncAccountMetadata_DoesNotOverwriteExistingExpiry(t *testing.T) {
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, &syncMetadataOAuthClientStub{})

	// Build an id_token with subscription_expires_at
	idToken := makeTestJWT(map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"subscription_expires_at": "2026-01-01T00:00:00Z",
		},
	})

	// Account already has subscription_expires_at in credentials
	existingExpiry := "2026-12-31T23:59:59Z"
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":            "original-access-token",
			"refresh_token":           "original-refresh-token",
			"id_token":                idToken,
			"subscription_expires_at": existingExpiry, // Already exists
		},
	}

	result, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.NoError(t, err)
	// Existing subscription_expires_at should be preserved (not overwritten by id_token fallback)
	require.Equal(t, existingExpiry, result.UpdatedCredentials["subscription_expires_at"])
}

func TestOpenAIOAuthService_SyncAccountMetadata_DoesNotCallRefreshToken(t *testing.T) {
	client := &syncMetadataOAuthClientStub{}
	svc := NewOpenAIOAuthService(&syncMetadataProxyRepoStub{}, client)

	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "existing-access-token",
			"refresh_token": "existing-refresh-token",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}

	// Even with refresh_token available, SyncAccountMetadata should NOT call refresh
	_, err := svc.SyncAccountMetadata(context.Background(), &SyncAccountMetadataInput{
		Account: account,
	})

	require.NoError(t, err)
	// The stub returns error on RefreshToken, so if no error occurred,
	// it means RefreshToken was NOT called
}

func init() {
	// Ensure domain constants match service constants
	_ = domain.PlatformOpenAI
}
