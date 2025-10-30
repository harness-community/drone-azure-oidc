// Copyright 2020 the Drone Authors. All rights reserved.

package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyEnv(t *testing.T) {
	tests := []struct {
		name    string
		args    Args
		wantErr bool
	}{
		{
			name: "missing oidc-token",
			args: Args{
				OIDCToken: "",
				TenantID:  "tenant-id",
				ClientID:  "client-id",
			},
			wantErr: true,
		},
		{
			name: "missing tenant-id",
			args: Args{
				OIDCToken: "oidc-token",
				TenantID:  "",
				ClientID:  "client-id",
			},
			wantErr: true,
		},
		{
			name: "missing client-id",
			args: Args{
				OIDCToken: "oidc-token",
				TenantID:  "tenant-id",
				ClientID:  "",
			},
			wantErr: true,
		},
		{
			name: "all args provided",
			args: Args{
				OIDCToken: "oidc-token",
				TenantID:  "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
				ClientID:  "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyEnv(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateGUID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid guid", value: "12345678-1234-1234-1234-1234567890ab", wantErr: false},
		{name: "missing dashes", value: "123456781234123412341234567890ab", wantErr: true},
		{name: "too short", value: "1234", wantErr: true},
		{name: "wrong dash positions", value: "12345678-1234-1234-123412-34567890ab", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGUID(tt.value, "field")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGUID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteEnvToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.env")
	t.Setenv("HARNESS_OUTPUT_SECRET_FILE", outPath)

	if err := WriteEnvToFile("AZURE_ACCESS_TOKEN", "test-token"); err != nil {
		t.Fatalf("WriteEnvToFile returned error: %v", err)
	}

	// verify contents
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed reading output file: %v", err)
	}
	got := string(data)
	wantLine := "AZURE_ACCESS_TOKEN=test-token\n"
	if !strings.Contains(got, wantLine) {
		t.Fatalf("output file missing line; got=%q want to contain %q", got, wantLine)
	}
}

func TestExchangeOIDCForAzureToken_Success(t *testing.T) {
	tenantID := "mytenant"
	clientID := "12345678-1234-1234-1234-1234567890ab"
	oidcToken := "oidc-token"

	// mock azure token endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// verify method and path
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		expectedPath := "/" + tenantID + "/oauth2/v2.0/token"
		if r.URL.Path != expectedPath {
			t.Fatalf("unexpected path: got %s want %s", r.URL.Path, expectedPath)
		}

		// verify form fields
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm error: %v", err)
		}
		if got := r.PostFormValue("client_id"); got != clientID {
			t.Fatalf("client_id mismatch: got %s", got)
		}
		if got := r.PostFormValue("client_assertion_type"); got != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" {
			t.Fatalf("client_assertion_type mismatch: %s", got)
		}
		if got := r.PostFormValue("client_assertion"); got != oidcToken {
			t.Fatalf("client_assertion mismatch: %s", got)
		}
		if got := r.PostFormValue("grant_type"); got != "client_credentials" {
			t.Fatalf("grant_type mismatch: %s", got)
		}
		// scope defaults when empty
		if got := r.PostFormValue("scope"); got != defaultScope {
			t.Fatalf("scope mismatch: got %q want %q", got, defaultScope)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token_type":"Bearer","expires_in":3600,"access_token":"abc"}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	token, err := ExchangeOIDCForAzureToken(ctx, oidcToken, tenantID, clientID, "", srv.URL)
	if err != nil {
		t.Fatalf("ExchangeOIDCForAzureToken returned error: %v", err)
	}
	if token == nil || token.AccessToken != "abc" || token.ExpiresIn != 3600 {
		t.Fatalf("unexpected token response: %+v", token)
	}
}

func TestExchangeOIDCForAzureToken_ErrorResponse(t *testing.T) {
	tenantID := "mytenant"
	clientID := "12345678-1234-1234-1234-1234567890ab"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"some long description","error_codes":[700016]}`))
	}))
	defer srv.Close()

	_, err := ExchangeOIDCForAzureToken(context.Background(), "id-token", tenantID, clientID, "custom.scope/.default", srv.URL)
	if err == nil || !strings.Contains(err.Error(), "token exchange failed") {
		t.Fatalf("expected token exchange failure, got %v", err)
	}
}

func TestExchangeOIDCForAzureToken_BadJSON(t *testing.T) {
	tenantID := "mytenant"
	clientID := "12345678-1234-1234-1234-1234567890ab"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":`)) // malformed JSON
	}))
	defer srv.Close()

	_, err := ExchangeOIDCForAzureToken(context.Background(), "id-token", tenantID, clientID, defaultScope, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "failed to decode response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}
