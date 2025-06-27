# AKS Mentions Bot

A comprehensive monitoring bot that tracks mentions of Azure Kubernetes Service (AKS) across various platforms including social media, forums, and blogs.
This project uses template files that you customize for your deployment:

### Configuration Process

1. **Run setup script**: `./setup.sh`
   - Creates `.env`, `k8s/*.local.yaml`, and `infra/*.local.json` from templates
   - These local files are gitignored to protect your secrets

2. **Edit the local files** with your values:
   - **`.env`**: Teams webhook, email settings, API keys
   - **`k8s/deployment.local.yaml`**: Your ACR name, storage account, etc.
   - **`k8s/secrets.local.yaml`**: Your Key Vault name, managed identity ID, tenant ID

3. **Deploy**: `azd up` does everything automatically:
   - Reads your local configuration files
   - Creates Azure infrastructure using Bicep templates
   - Builds and pushes container image to your ACR
   - Deploys to Kubernetes using your customized manifests

> **🔒 Security**: Template files (`.template.*`) contain placeholders and are safe for git. Local files (`.local.*`) contain your real values and are gitignored.


## 🚀 Quick Start

### Option 1: Deploy to AKS (Recommended)

```bash
# 1. Clone and setup
git clone <your-repository-url>
cd aks-mentions-bot

# 2. Setup configuration files from templates
./setup.sh
# This creates .env, k8s/*.local.yaml, and infra/*.local.json from templates

# 3. Edit your configuration with real values
# Edit .env with basic settings (Teams webhook URL, etc.)
# Edit k8s/deployment.local.yaml with your Azure resource names
# Edit k8s/secrets.local.yaml with your Key Vault and identity details

# 4. Deploy everything with one command
az login && azd auth login
azd up
# AZD automatically:
# - Creates infrastructure (AKS, ACR, Key Vault, Storage, etc.)
# - Builds and pushes container image to ACR
# - Deploys to Kubernetes using your .local.yaml files

# 5. Add API secrets to Key Vault (optional - bot works without them)
az keyvault secret set --vault-name <your-keyvault> --name teams-webhook-url --value "<your-webhook>"
az keyvault secret set --vault-name <your-keyvault> --name reddit-client-id --value "<your-reddit-id>"
# Add other API keys as needed...

# 6. Test it works
kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot -f
```

### Option 2: Local Development

```bash
# Clone and setup
git clone <your-repository-url>
cd aks-mentions-bot

# Copy template files to local versions (these are gitignored)
cp .env.example .env
cp k8s/deployment.template.yaml k8s/deployment.local.yaml
cp k8s/secrets.template.yaml k8s/secrets.local.yaml
cp infra/main.parameters.template.json infra/main.parameters.local.json

# Edit .env and local files with your actual API keys and configuration
# Note: .local.* files are in .gitignore to protect your secrets

# Test API connectivity
make test-apis

# Run locally
make run
```

> **🔒 Security Note**: This project uses template files (`.template.*`) that get copied to local versions (`.local.*`). The local versions contain your actual secrets and are excluded from git via `.gitignore`. Always use the local versions for deployment and never commit files with real secrets.

## � Configuration Files

This project uses template files that you customize for your deployment:

### Configuration Process

1. **Run setup script**: `./setup.sh` 
   - Creates `.env`, `k8s/*.local.yaml`, and `infra/*.local.json` from templates
   - These local files are gitignored to protect your secrets

2. **Edit the local files** with your values:
   - **`.env`**: Teams webhook, email settings, API keys
   - **`k8s/deployment.local.yaml`**: Your ACR name, storage account, etc.
   - **`k8s/secrets.local.yaml`**: Your Key Vault name, managed identity ID, tenant ID

3. **Deploy**: `azd up` uses your local configuration files

> **🔒 Security**: Template files (`.template.*`) contain placeholders and are safe for git. Local files (`.local.*`) contain your real values and are gitignored.


## 🔧 Technology Stack

- **Language**: Go 1.21+
- **Deployment**: Azure Kubernetes Service (AKS)
- **Infrastructure**: Azure Bicep templates
- **Storage**: Azure Blob Storage
- **Identity**: Azure Workload Identity
- **Container Registry**: Azure Container Registry
- **Monitoring**: Azure Application Insights

## ⚙️ Configuration

All configuration is done through environment variables:

### Required Settings

- `TEAMS_WEBHOOK_URL`: Microsoft Teams webhook URL (or use email)
- `NOTIFICATION_EMAIL`: Email address to send reports to (or use Teams)
- `AZURE_STORAGE_ACCOUNT`: Azure Storage account name for data persistence

### Optional Settings

- `REPORT_SCHEDULE`: "daily" or "weekly" (default: weekly)
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`: Email configuration (required if using email notifications)
- `KEYWORDS`: Comma-separated list of keywords to monitor (default: "Azure Kubernetes Service,AKS")

### API Keys (Optional - sources are disabled if not provided)

- `REDDIT_CLIENT_ID` and `REDDIT_CLIENT_SECRET`: Reddit API credentials
- `TWITTER_BEARER_TOKEN`: Twitter API v2 Bearer Token
- `YOUTUBE_API_KEY`: YouTube Data API v3 key

## 💻 Local Development

```bash
# Clone repository
git clone <your-repository-url>
cd aks-mentions-bot

# Copy environment template
cp .env.example .env
# Edit .env with your API keys and webhook URLs

# Install dependencies
go mod tidy

# Run tests
make test

# Test API connectivity  
make test-apis

# Run locally
make run

# Generate test report
make test-report-cli

# Run integration tests
make test-integration
```

## � Testing and Troubleshooting

### Quick Test

```bash
# For AKS deployment
kubectl port-forward service/aks-mentions-bot-service 8080:80 -n aks-mentions-bot

# Test endpoints
curl http://localhost:8080/health
curl -X POST http://localhost:8080/trigger  # Manual run
```

### Check Logs

```bash
# For AKS deployment
kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot

# Check pod status  
kubectl get pods -n aks-mentions-bot
```

### Common Issues

- **Missing API keys**: Only Reddit, Twitter/X, and YouTube require API keys
- **Teams webhook not working**: Check the webhook URL is correct
- **No mentions found**: Run `make test-apis` to verify source connectivity
- **Pod not starting**: Check `kubectl describe pod -n aks-mentions-bot`
- **Confused about .env vs secrets**: Use `.env` for local dev, Kubernetes secrets for AKS deployment

## �📊 What to Expect

### Reports Include

- **Total mentions** found across all sources
- **Breakdown by source** (Reddit, Twitter, YouTube, etc.)
- **Sentiment analysis** (positive, negative, neutral)  
- **Top sources** with most mentions
- **Sample mentions** with titles and links

### Default Behavior

- **Runs**: Every Monday 9 AM UTC (configurable)
- **Keywords**: "AKS", "Azure Kubernetes Service" (configurable)
- **Context Filtering**: Filters out weapon-related AKS mentions
- **Storage**: All data saved to Azure Blob Storage



## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🆘 Need Help?

- Try Copilot

