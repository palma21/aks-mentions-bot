# 🚀 AKS Mentions Bot - Complete Deployment Guide

This guide will walk you through deploying and configuring the AKS Mentions Bot from start to finish.

## 📋 Prerequisites

1. **Azure CLI** - [Install here](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
2. **Azure Developer CLI (azd)** - [Install here](https://learn.microsoft.com/en-us/azure/developer/azure-developer-cli/install-azd)
3. **kubectl** - [Install here](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
4. **Docker** - [Install here](https://docs.docker.com/get-docker/)
5. **Git** - [Install here](https://git-scm.com/downloads)

## 🔐 Step 1: Get API Keys and Tokens

### Reddit API Keys
1. Go to [Reddit App Preferences](https://www.reddit.com/prefs/apps)
2. Click "Create App" or "Create Another App"
3. Fill in:
   - **Name**: `AKS Mentions Bot`
   - **App type**: `script`
   - **Description**: `Bot to monitor AKS mentions`
   - **About URL**: Leave blank
   - **Redirect URI**: `http://localhost`
4. Click "Create app"
5. Note down:
   - **Client ID**: The string under the app name
   - **Client Secret**: The "secret" field

### Twitter/X API Keys
1. Go to [Twitter Developer Portal](https://developer.twitter.com/)
2. Apply for a developer account if you don't have one
3. Create a new app in your developer portal
4. Go to the app's "Keys and tokens" tab
5. Generate and note down:
   - **Bearer Token**: This is what you need for the bot

### YouTube API Key
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing one
3. Enable the **YouTube Data API v3**
4. Go to "Credentials" → "Create Credentials" → "API Key"
5. Note down the **API Key**
6. (Optional) Restrict the key to YouTube Data API v3

### Optional APIs
- **Stack Overflow**: No API key needed (uses public API)
- **Hacker News**: No API key needed (uses public API)
- **Medium**: Uses RSS feeds (no API key needed)
- **LinkedIn**: Uses public search APIs (no API key needed)

## 📧 Step 2: Set Up Teams Webhook

### Create Teams Webhook
1. Open Microsoft Teams
2. Go to the channel where you want notifications
3. Click the **"..." (More options)** next to the channel name
4. Select **"Connectors"**
5. Find **"Incoming Webhook"** and click **"Add"**
6. Click **"Add"** again, then **"Configure"**
7. Give it a name: `AKS Mentions Bot`
8. (Optional) Upload an icon
9. Click **"Create"**
10. **Copy the webhook URL** - it looks like:
    ```
    https://your-org.webhook.office.com/webhookb2/your-webhook-id/IncomingWebhook/your-channel-id/your-secret
    ```

### Alternative: Email Notifications
If you prefer email notifications instead of Teams:

1. **SMTP Settings for Office 365/Outlook**:
   - SMTP Host: `smtp.office365.com`
   - SMTP Port: `587`
   - Username: Your email address
   - Password: App password (not your regular password)

2. **Generate App Password**:
   - Go to [Microsoft Account Security](https://account.microsoft.com/security)
   - Sign in and go to "Security dashboard"
   - Select "Advanced security options"
   - Under "App passwords", select "Create a new app password"
   - Note down the generated password

3. **Gmail SMTP** (if using Gmail):
   - SMTP Host: `smtp.gmail.com`
   - SMTP Port: `587`
   - Username: Your Gmail address
   - Password: App password (enable 2FA first)

## 🚀 Step 3: Deploy Infrastructure Only

### Login to Azure
```bash
# Login to Azure
az login

# Login to Azure Developer CLI
azd auth login
```

### Initialize and Provision Infrastructure
```bash
# Navigate to project directory
cd aks-mentions-bot

# Initialize azd (if not already done)
azd init

# Deploy infrastructure only (without application)
azd provision
```

The `azd provision` command will:
1. Generate SSH keys for AKS automatically
2. Provision all Azure resources (AKS, ACR, Storage, etc.)
3. Set up workload identity
4. **NOT** deploy the application yet

### Get AKS Credentials
```bash
# Get AKS credentials for kubectl
azd env get-value AKS_CLUSTER_NAME
az aks get-credentials --resource-group $(azd env get-value AZURE_RESOURCE_GROUP) --name $(azd env get-value AKS_CLUSTER_NAME)
```

## ⚙️ Step 4: Configure API Keys and Notifications BEFORE App Deployment

> **⚠️ WARNING**: Kubernetes secrets are not encrypted. Use Azure Key Vault for production.

### Production: Azure Key Vault
```bash
# Create Key Vault
az keyvault create --name "aks-mentions-keyvault-$(date +%s)" \
  --resource-group $(azd env get-value AZURE_RESOURCE_GROUP) \
  --location $(azd env get-value AZURE_LOCATION)

# Store secrets
az keyvault secret set --vault-name "your-keyvault-name" --name "teams-webhook-url" --value "https://your-webhook-url"
az keyvault secret set --vault-name "your-keyvault-name" --name "reddit-client-id" --value "your-reddit-id"
# (Requires app code changes to use Azure SDK)
```

### Development/Testing: Kubernetes Secrets
```bash
# Copy template
cp k8s/secrets.yaml k8s/my-secrets.yaml

# Edit k8s/my-secrets.yaml with your values, then apply
kubectl apply -f k8s/my-secrets.yaml
```

## 🚀 Step 5: Deploy Application

Now that infrastructure is provisioned and secrets are configured, deploy the application:

```bash
# Deploy the application to the existing infrastructure
azd deploy
```

### Monitor Deployment
```bash
# Check deployment status
azd show

# Get AKS credentials for kubectl
azd env get-value AKS_CLUSTER_NAME
az aks get-credentials --resource-group $(azd env get-value AZURE_RESOURCE_GROUP) --name $(azd env get-value AKS_CLUSTER_NAME)

# Check if pods are running
kubectl get pods -n aks-mentions-bot

# View logs
kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot -f
```

## 🧪 Step 6: Test the Bot

### Test API Connectivity
```bash
# Port forward to access the bot locally
kubectl port-forward service/aks-mentions-bot-service 8080:80 -n aks-mentions-bot

# In another terminal, test the APIs
curl http://localhost:8080/health
curl http://localhost:8080/metrics

# Manually trigger a monitoring run
curl -X POST http://localhost:8080/trigger
```

### Test from Local Machine (Alternative)
```bash
# Build and run locally to test API connectivity
make test-apis

# Run integration test
make test-integration

# Generate a test report
make test-report-cli
```

## 📊 Step 7: Configure Monitoring Schedule

The bot is configured to run **weekly** by default (every Monday at 9 AM UTC). You can modify this:

### Change Schedule
Edit the ConfigMap:
```bash
kubectl edit configmap aks-mentions-bot-config -n aks-mentions-bot
```

Change `REPORT_SCHEDULE` to:
- `daily` - For daily reports
- `weekly` - For weekly reports (default)

### Manual Triggers
```bash
# Trigger a manual run
kubectl exec -it deployment/aks-mentions-bot -n aks-mentions-bot -- curl -X POST http://localhost:8080/trigger
```

## 🔍 Step 8: Monitor and Troubleshoot

### View Logs
```bash
# Stream live logs
kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot -f

# Get recent logs
kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot --tail=100

# Get logs from Azure Monitor (if configured)
# Go to Azure Portal → Your AKS cluster → Insights → Container logs
```

### Check Status
```bash
# Check all resources
kubectl get all -n aks-mentions-bot

# Check pod details
kubectl describe pod -l app=aks-mentions-bot -n aks-mentions-bot

# Check secrets (without showing values)
kubectl get secrets -n aks-mentions-bot
```

### Troubleshooting

**Rate Limits**: Check logs for quota exceeded errors  
**Auth Errors**: Verify API keys in secrets  
**Teams Not Working**: Verify webhook URL is active  
**Email Not Sending**: Check SMTP settings and app passwords

## 🎯 Step 8: Expected Behavior

### Weekly Reports
- Bot runs every **Monday at 9 AM UTC**
- Collects mentions from the **previous 7 days**
- Sends report to Teams channel and/or email
- Stores data in Azure Blob Storage

### Report Contents
- **Total mentions** found across all sources
- **Breakdown by source** (Reddit, Twitter, YouTube, etc.)
- **Sentiment analysis** (positive, negative, neutral)
- **Top sources** with most mentions
- **Sample mentions** with titles and links

### Data Sources Monitored
- **Reddit**: Subreddits and posts mentioning AKS
- **Stack Overflow**: Questions about AKS
- **Hacker News**: Stories mentioning AKS
- **Twitter/X**: Tweets about AKS
- **YouTube**: Videos mentioning AKS
- **Medium**: Articles and posts mentioning AKS (via RSS feeds)
- **LinkedIn**: Public content mentioning AKS (via search APIs)

## 🔧 Step 9: Scaling and Updates

### Scale the Application
```bash
# Scale to 2 replicas
kubectl scale deployment aks-mentions-bot --replicas=2 -n aks-mentions-bot

# Enable auto-scaling
kubectl autoscale deployment aks-mentions-bot --cpu-percent=70 --min=1 --max=3 -n aks-mentions-bot
```

### Update the Application
```bash
# Make code changes, then redeploy
azd deploy

# Or build and update manually
make docker-build
docker tag aks-mentions-bot:latest $(azd env get-value AZURE_CONTAINER_REGISTRY_ENDPOINT)/aks-mentions-bot:latest
docker push $(azd env get-value AZURE_CONTAINER_REGISTRY_ENDPOINT)/aks-mentions-bot:latest
kubectl rollout restart deployment/aks-mentions-bot -n aks-mentions-bot
```

## 🧹 Cleanup

To remove everything:
```bash
# Delete Azure resources
azd down

# Or just delete the application
kubectl delete namespace aks-mentions-bot
```

## 🆘 Support

If you encounter issues:

1. **Check logs**: `kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot`
2. **Test locally**: `make test-apis` and `make test-integration`
3. **Verify configuration**: Ensure all API keys and webhooks are correct
4. **Monitor Azure resources**: Check Azure Portal for any resource issues

The bot should now be running and monitoring for AKS mentions across multiple platforms!
