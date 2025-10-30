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

// default settings for Azure authority and HTTP
const (
	defaultAuthorityHost = "https://login.microsoftonline.com"
	defaultHTTPTimeout   = 30 * time.Second
	defaultScope         = "https://management.azure.com/.default"
)

// ExchangeOIDCForAzureToken exchanges an external OIDC token for an Azure AD access token.
func ExchangeOIDCForAzureToken(ctx context.Context, oidcToken, tenantID, clientID, scope, authorityHost string) (*AzureTokenResponse, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, defaultHTTPTimeout)
	defer cancel()

	// Apply default values if not provided
	if strings.TrimSpace(authorityHost) == "" {
		authorityHost = defaultAuthorityHost
	}
	if strings.TrimSpace(scope) == "" {
		scope = defaultScope
	}
	authorityHost = strings.TrimRight(authorityHost, "/")
	tokenEndpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/token", authorityHost, tenantID)

	logrus.Debugf("token endpoint: %s", tokenEndpoint)
	logrus.Debugf("client_id: %s", clientID)
	logrus.Debugf("scope: %s", scope)
	logrus.Debugf("azure_authority_host: %s", authorityHost)

	// Prepare request body
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scope)
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", oidcToken)
	data.Set("grant_type", "client_credentials")

	// Make HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	if resp.StatusCode != http.StatusOK {
		// Limit error body to avoid logging large payloads
		var azureErr AzureErrorResponse
		limited := &io.LimitedReader{R: resp.Body, N: 4096}
		_ = json.NewDecoder(limited).Decode(&azureErr)
		if azureErr.Error != "" {
			return nil, fmt.Errorf("token exchange failed: %s - %s (status=%d)", azureErr.Error, sanitizeErrorDescription(azureErr.ErrorDescription), resp.StatusCode)
		}
		return nil, fmt.Errorf("token exchange failed: %s", resp.Status)
	}

	var tokenResp AzureTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tokenResp, nil
}

// sanitizeErrorDescription removes potentially sensitive information from error messages.
func sanitizeErrorDescription(desc string) string {
	// Azure error descriptions are generally safe, but truncate if too long
	if len(desc) > 200 {
		return desc[:200] + "..."
	}
	return desc
}
