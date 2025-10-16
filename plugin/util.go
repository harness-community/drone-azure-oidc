// Copyright 2020 the Drone Authors. All rights reserved.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// AzureTokenResponse represents the response from Azure AD token endpoint.
type AzureTokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// AzureErrorResponse represents an error response from Azure AD.
type AzureErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	ErrorCodes       []int  `json:"error_codes"`
	Timestamp        string `json:"timestamp"`
	TraceID          string `json:"trace_id"`
	CorrelationID    string `json:"correlation_id"`
}

// ExchangeOIDCForAzureToken exchanges an external OIDC token for an Azure AD access token.
func ExchangeOIDCForAzureToken(ctx context.Context, oidcToken, tenantID, clientID, scope string) (*AzureTokenResponse, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Validate inputs
	if err := validateGUID(tenantID, "tenant_id"); err != nil {
		return nil, err
	}
	if err := validateGUID(clientID, "client_id"); err != nil {
		return nil, err
	}

	// Construct Azure AD token endpoint
	tokenEndpoint := fmt.Sprintf(
		"https://login.microsoftonline.com/%s/oauth2/v2.0/token",
		tenantID,
	)

	logrus.Debugf("token endpoint: %s", tokenEndpoint)
	logrus.Debugf("client_id: %s", clientID)
	logrus.Debugf("scope: %s", scope)

	// Prepare request body
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scope)
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", oidcToken)
	data.Set("grant_type", "client_credentials")

	// Make HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		tokenEndpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	if resp.StatusCode != http.StatusOK {
		// Try to parse Azure error response
		var azureErr AzureErrorResponse
		if err := json.Unmarshal(body, &azureErr); err == nil && azureErr.Error != "" {
			return nil, fmt.Errorf(
				"token exchange failed: %s - %s (trace_id: %s)",
				azureErr.Error,
				sanitizeErrorDescription(azureErr.ErrorDescription),
				azureErr.TraceID,
			)
		}
		// Fallback to generic error (sanitize to avoid leaking tokens)
		return nil, fmt.Errorf("token exchange failed: %s", resp.Status)
	}

	var tokenResp AzureTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tokenResp, nil
}

// validateGUID validates that a string is a valid GUID format.
func validateGUID(value, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	// Basic GUID validation (8-4-4-4-12 format)
	if len(value) == 36 && value[8] == '-' && value[13] == '-' && value[18] == '-' && value[23] == '-' {
		return nil
	}
	return fmt.Errorf("%s must be a valid GUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)", fieldName)
}

// sanitizeErrorDescription removes potentially sensitive information from error messages.
func sanitizeErrorDescription(desc string) string {
	// Azure error descriptions are generally safe, but truncate if too long
	if len(desc) > 200 {
		return desc[:200] + "..."
	}
	return desc
}
