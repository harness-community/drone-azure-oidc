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
	Level         string `envconfig:"PLUGIN_LOG_LEVEL"`
	OIDCToken     string `envconfig:"PLUGIN_OIDC_TOKEN_ID"`
	TenantID      string `envconfig:"PLUGIN_TENANT_ID"`
	ClientID      string `envconfig:"PLUGIN_CLIENT_ID"`
	Scope         string `envconfig:"PLUGIN_SCOPE"`
	AuthorityHost string `envconfig:"PLUGIN_AZURE_AUTHORITY_HOST"`
}

// Exec executes the plugin.
func Exec(ctx context.Context, args Args) error {
	// 1. verify Env variables
	if err := VerifyEnv(args); err != nil {
		return err
	}
	// 2. Exchange OIDC token for Azure AD access token
	logrus.Infof("exchanging OIDC token for Azure AD access token")
	tokenResp, err := ExchangeOIDCForAzureToken(
		ctx,
		args.OIDCToken,
		args.TenantID,
		args.ClientID,
		args.Scope,
		args.AuthorityHost,
	)
	if err != nil {
		return fmt.Errorf("failed to exchange OIDC token: %w", err)
	}
	// 3. Write access token to output file
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
	if err := validateGUID(args.TenantID, "tenant-id"); err != nil {
		return err
	}
	if err := validateGUID(args.ClientID, "client-id"); err != nil {
		return err
	}
	return nil
}

func validateGUID(value, fieldName string) error {
	if len(value) == 36 && value[8] == '-' && value[13] == '-' && value[18] == '-' && value[23] == '-' {
		return nil
	}
	return fmt.Errorf("%s must be a valid GUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)", fieldName)
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
