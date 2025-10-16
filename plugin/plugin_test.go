// Copyright 2020 the Drone Authors. All rights reserved.

package plugin

import "testing"

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
				TenantID:  "tenant-id",
				ClientID:  "client-id",
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

func TestWriteEnvToFile(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{
			name:    "write to file",
			key:     "AZURE_ACCESS_TOKEN",
			value:   "test-token",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteEnvToFile(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteEnvToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
