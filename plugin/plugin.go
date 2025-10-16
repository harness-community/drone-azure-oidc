// Copyright 2020 the Drone Authors. All rights reserved.

package plugin

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// Args provides plugin execution arguments.
type Args struct {
	Pipeline

	// Level defines the plugin log level.
	Level string `envconfig:"PLUGIN_LOG_LEVEL"`

	// OIDCToken is the OIDC token from Harness (auto-populated by Harness CI).
	OIDCToken string `envconfig:"PLUGIN_OIDC_TOKEN_ID"`

	// TenantID is the Azure AD Tenant ID.
	TenantID string `envconfig:"PLUGIN_TENANT_ID"`

	// ClientID is the Azure AD Application (Client) ID.
	ClientID string `envconfig:"PLUGIN_CLIENT_ID"`

	// Scope is the Azure Resource Scope (optional, defaults to Azure Storage).
	Scope string `envconfig:"PLUGIN_SCOPE"`
}

// Exec executes the plugin.
func Exec(ctx context.Context, args Args) error {
	// 1. Validate required arguments
	if err := VerifyEnv(args); err != nil {
		return err
	}

	// 2. Set default scope if not provided
	if args.Scope == "" {
		args.Scope = "https://storage.azure.com/.default"
		logrus.Debugf("using default scope: %s", args.Scope)
	}

	// 3. Exchange OIDC token for Azure AD access token
	logrus.Infof("exchanging OIDC token for Azure AD access token")
	tokenResp, err := ExchangeOIDCForAzureToken(
		ctx,
		args.OIDCToken,
		args.TenantID,
		args.ClientID,
		args.Scope,
	)
	if err != nil {
		return fmt.Errorf("failed to exchange OIDC token: %w", err)
	}

	// 4. Write access token to output file
	if err := WriteEnvToFile("AZURE_ACCESS_TOKEN", tokenResp.AccessToken); err != nil {
		return err
	}

	logrus.Infof("Azure access token retrieved successfully")
	logrus.Debugf("token will expire in %d seconds", tokenResp.ExpiresIn)

	return nil
}

// VerifyEnv validates that all required environment variables are provided.
func VerifyEnv(args Args) error {
	if args.OIDCToken == "" {
		return fmt.Errorf("oidc-token is not provided")
	}
	if args.TenantID == "" {
		return fmt.Errorf("tenant-id is not provided")
	}
	if args.ClientID == "" {
		return fmt.Errorf("client-id is not provided")
	}
	return nil
}

// WriteEnvToFile writes a key-value pair to the Harness output secret file.
func WriteEnvToFile(key, value string) error {
	outputFile, err := os.OpenFile(
		os.Getenv("HARNESS_OUTPUT_SECRET_FILE"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer outputFile.Close()

	_, err = fmt.Fprintf(outputFile, "%s=%s\n", key, value)
	if err != nil {
		return fmt.Errorf("failed to write to env: %w", err)
	}

	return nil
}
