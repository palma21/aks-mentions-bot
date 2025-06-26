# ⚡ Quick Start - AKS Mentions Bot

## 🚀 Deploy in 5 Minutes

> **Note**: This guide is for **AKS deployment**. For local development, copy `.env.example` to `.env` and configure it instead.

```bash
# 1. Login to Azure
az login && azd auth login

# 2. Deploy everything
cd aks-mentions-bot
azd up

# 3. Configure secrets - Choose ONE approach:

# ⚠️ SECURITY NOTE: For production, use Azure Key Vault instead!
# Kubernetes secrets are not encrypted and visible to cluster admins.

# Option A: Edit k8s/secrets.yaml and apply (Development)
cp k8s/secrets.yaml k8s/my-secrets.yaml
# Edit k8s/my-secrets.yaml with your actual values
kubectl apply -f k8s/my-secrets.yaml

# Option B: Create secrets directly with kubectl (Testing only)
kubectl create secret generic aks-mentions-bot-secrets \
  --from-literal=TEAMS_WEBHOOK_URL="https://your-teams-webhook" \
  --from-literal=REDDIT_CLIENT_ID="your-reddit-id" \
  --from-literal=REDDIT_CLIENT_SECRET="your-reddit-secret" \
  --from-literal=TWITTER_BEARER_TOKEN="your-twitter-token" \
  --from-literal=YOUTUBE_API_KEY="your-youtube-key" \
  --namespace=aks-mentions-bot

# 4. Restart to apply config
kubectl rollout restart deployment/aks-mentions-bot -n aks-mentions-bot

# 5. Test it works
kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot -f
```

## 📋 Where to Get API Keys

### Required API Keys
| Service | Where to Get | What You Need |
|---------|-------------|---------------|
| **Reddit** | [reddit.com/prefs/apps](https://reddit.com/prefs/apps) | Client ID + Secret |
| **Twitter/X** | [developer.twitter.com](https://developer.twitter.com) | Bearer Token |
| **YouTube** | [console.cloud.google.com](https://console.cloud.google.com) | API Key |
| **Teams** | Teams Channel → Connectors → Incoming Webhook | Webhook URL |

### No API Keys Needed
✅ **Stack Overflow** - Uses public API  
✅ **Hacker News** - Uses public API  
✅ **Medium** - Uses RSS feeds  
✅ **LinkedIn** - Uses public search

## 🔍 Quick Test

```bash
# Port forward and test
kubectl port-forward service/aks-mentions-bot-service 8080:80 -n aks-mentions-bot

# Test endpoints
curl http://localhost:8080/health
curl -X POST http://localhost:8080/trigger  # Manual run
```

## 📊 What to Expect

- **Runs**: Every Monday 9 AM UTC
- **Reports**: Teams webhook + email (if configured)  
- **Monitors**: Reddit, Twitter, YouTube, Stack Overflow, Hacker News, Medium, LinkedIn
- **Keywords**: "AKS", "Azure Kubernetes Service", "KubeFleet", "KAITO"

## 🆘 Troubleshooting

```bash
# Check logs
kubectl logs -l app=aks-mentions-bot -n aks-mentions-bot

# Check pod status  
kubectl get pods -n aks-mentions-bot

# Test all APIs locally
make test-apis

# Run full integration test
make test-integration

# Generate test report
make test-report-cli
```

### Common Issues
- **Missing API keys**: Only Reddit, Twitter/X, and YouTube require API keys
- **Teams webhook not working**: Check the webhook URL is correct
- **No mentions found**: Run `make test-apis` to verify source connectivity
- **Pod not starting**: Check `kubectl describe pod -n aks-mentions-bot`
- **Confused about .env vs secrets**: Use `.env` for local dev, Kubernetes secrets for AKS deployment

See **DEPLOYMENT_GUIDE.md** for detailed instructions!
