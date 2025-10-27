# drone-azure-oidc

A Drone plugin that exchanges Harness OIDC tokens for Azure AD access tokens using Service Principal authentication with Federated Identity Credentials.

## Synopsis

This plugin generates an Azure AD access token through OIDC token exchange and outputs it as an environment variable. This variable can be utilized in subsequent pipeline steps to access Azure services like Azure Storage, Azure Container Registry, and more.

To learn how to utilize Drone plugins in Harness CI, please consult the [documentation](https://developer.harness.io/docs/continuous-integration/use-ci/use-drone-plugins/run-a-drone-plugin-in-ci).

## Authentication Method

**Service Principal (App Registration) with Federated Identity Credentials**

This plugin is designed exclusively for Service Principal authentication, which is the standard pattern for enterprise CI/CD pipelines. User-Assigned Managed Identity is NOT supported as it requires running on Azure infrastructure and uses Azure Instance Metadata Service (IMDS) instead of OIDC token exchange.

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `tenant_id` | string | Yes | - | The Azure AD Tenant ID (GUID or one of: `common`, `organizations`, `consumers`) |
| `client_id` | string | Yes | - | The Azure AD Application (Client) ID (GUID) |
| `scope` | string | No | `https://management.azure.com/.default` | The Azure resource scope for the access token |
| `azure_authority_host` | string | No | `https://login.microsoftonline.com` | The Azure AD authority host to use (set for national clouds like Azure Gov/China) |

## Supported Scopes

| Service | Scope |
|---------|-------|
| Azure Management API (default) | `https://management.azure.com/.default` |
| Azure Storage | `https://storage.azure.com/.default` |
| Microsoft Graph | `https://graph.microsoft.com/.default` |
| Azure Container Registry | `https://containerregistry.azure.net/.default` |
| Azure Key Vault | `https://vault.azure.net/.default` |
| Azure Database | `https://database.windows.net/.default` |

**Important**: The scope determines which Azure service API the token is valid for. You must ALSO assign appropriate RBAC roles to the Service Principal in Azure to authorize specific operations.

## Notes

- `PLUGIN_OIDC_TOKEN_ID` is not manually configured; the Harness CI platform automatically generates and sets this environment variable when it detects the `drone-azure-oidc` plugin is being executed.

- The plugin outputs the access token in the form of an environment variable: `AZURE_ACCESS_TOKEN`

- This can be accessed in subsequent pipeline steps like: `<+steps.STEP_ID.output.outputVariables.AZURE_ACCESS_TOKEN>`

## Plugin Image

The plugin `plugins/azure-oidc` is available for the following architectures:

| OS | Tag |
|----|-----|
| latest | `linux-amd64/arm64, windows-amd64` |
| linux/amd64 | `linux-amd64` |
| linux/arm64 | `linux-arm64` |
| windows/amd64 | `windows-amd64` |

## Usage Examples

### Basic Authentication with Azure Management

```yaml
- step:
    type: Plugin
    name: Azure OIDC Authentication
    identifier: azure_oidc_auth
    spec:
      connectorRef: harness-docker-connector
      image: plugins/azure-oidc
      settings:
        tenant_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
        client_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

### Authentication with Azure Container Registry

```yaml
- step:
    type: Plugin
    name: Azure OIDC for ACR
    identifier: azure_oidc_acr
    spec:
      connectorRef: harness-docker-connector
      image: plugins/azure-oidc
      settings:
        tenant_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
        client_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
        scope: https://containerregistry.azure.net/.default
```

### Custom Authority Host (Azure Government example)

```yaml
- step:
    type: Plugin
    name: Azure OIDC (US Gov)
    identifier: azure_oidc_usgov
    spec:
      connectorRef: harness-docker-connector
      image: plugins/azure-oidc
      settings:
        tenant_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
        client_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
        scope: https://management.azure.us/.default
        azure_authority_host: https://login.microsoftonline.us
```

## Azure Prerequisites

Before using this plugin, you must configure Azure AD and RBAC permissions:

### 1. Create Azure AD Application

```bash
az ad app create \
  --display-name "harness-oidc-app" \
  --sign-in-audience AzureADMyOrg
```

Save the `Application (client) ID` from the output.

### 2. Create Service Principal

```bash
az ad sp create --id {application_id}
```

### 3. Configure Federated Identity Credential

```bash
az ad app federated-credential create \
  --id {application_id} \
  --parameters '{
    "name": "harness-federated-identity",
    "issuer": "https://app.harness.io/ng/api/oidc/account/{account_id}",
    "subject": "pipeline:{pipeline_id}",
    "description": "Federated identity for Harness CI pipeline",
    "audiences": ["api://AzureADTokenExchange"]
  }'
```

**Note**:
- Replace `{account_id}` with your Harness account ID
- Replace `{pipeline_id}` with your pipeline identifier or use wildcards like `pipeline:*` for all pipelines
- The `audiences` value must be `api://AzureADTokenExchange` (fixed for Azure workload identity federation)

### 4. Assign RBAC Permissions

Assign appropriate Azure RBAC roles to the Service Principal:

#### For Azure Storage Access

```bash
az role assignment create \
  --assignee {application_id} \
  --role "Storage Blob Data Contributor" \
  --scope /subscriptions/{subscription_id}/resourceGroups/{resource_group}/providers/Microsoft.Storage/storageAccounts/{storage_account}
```

#### For Azure Container Registry Access

```bash
az role assignment create \
  --assignee {application_id} \
  --role "AcrPush" \
  --scope /subscriptions/{subscription_id}/resourceGroups/{resource_group}/providers/Microsoft.ContainerRegistry/registries/{registry_name}
```

#### For Azure Management Operations

```bash
az role assignment create \
  --assignee {application_id} \
  --role "Contributor" \
  --scope /subscriptions/{subscription_id}/resourceGroups/{resource_group}
```

## How It Works

### Authentication Flow

```
1. Harness CI generates OIDC token
   ↓
2. Plugin receives OIDC token via PLUGIN_OIDC_TOKEN_ID
   ↓
3. Plugin exchanges OIDC token with Azure AD
   POST {azure_authority_host}/{tenant}/oauth2/v2.0/token (default: https://login.microsoftonline.com)
   ↓
4. Azure AD validates the token against Federated Identity Credential
   ↓
5. Azure AD returns access token
   ↓
6. Plugin writes AZURE_ACCESS_TOKEN to HARNESS_OUTPUT_SECRET_FILE
   ↓
7. Subsequent steps can use the access token
```

### Scope vs RBAC Permissions

Understanding the difference between scope and RBAC:

**Scope (Plugin Setting)**
- Determines which Azure API you can call
- Example: `https://storage.azure.com/.default`
- Set at token request time

**RBAC Role Assignment (Azure Configuration)**
- Determines what operations you can perform
- Example: "Storage Blob Data Contributor" role
- Configured in Azure portal or CLI

**Both are required** for successful authentication and authorization.

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `AADSTS700016: invalid client assertion` | Invalid OIDC token or wrong client ID | Verify client_id matches your Azure AD app |
| `AADSTS700024: Client assertion is not within its valid time range` | Federated credential not configured or expired OIDC token | Configure federated identity credential in Azure AD |
| `AADSTS90002: Tenant not found` | Invalid tenant ID | Verify tenant_id is correct GUID |
| `AADSTS70011: The provided scope is not valid` | Invalid or unauthorized scope | Check scope format and app permissions |
| `oidc-token is not provided` | Harness didn't generate OIDC token | Ensure plugin is running in Harness CI with OIDC enabled |

### Debug Mode

Enable debug logging to troubleshoot issues:

```yaml
- step:
    type: Plugin
    name: Azure OIDC Authentication
    identifier: azure_oidc_auth
    spec:
      connectorRef: harness-docker-connector
      image: plugins/azure-oidc
      settings:
        tenant_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
        client_id: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
        log_level: debug  # or 'trace' for more verbose output
```

## Building from Source

### Prerequisites

- Go 1.23 or later
- Docker (for building images)

### Build Binary

```bash
./scripts/build.sh
```

This will create binaries in the `release/` directory for:
- Linux (amd64, arm64)
- Windows (amd64)

### Build Docker Image

```bash
docker build -f docker/Dockerfile -t plugins/azure-oidc:latest .
```

### Run Tests

```bash
cd plugin
go test -v
```

## License

Apache License 2.0

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## Support

For issues and questions:
- GitHub Issues: [harness-community/drone-azure-oidc](https://github.com/harness-community/drone-azure-oidc/issues)
- Harness Documentation: [developer.harness.io](https://developer.harness.io)
