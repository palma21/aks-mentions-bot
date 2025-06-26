# AKS Mentions Bot

A comprehensive monitoring bot that tracks mentions of Azure Kubernetes Service (AKS), Azure Kubernetes Fleet Manager, KubeFleet OSS, and KAITO OSS across various platforms including social media, forums, and blogs.

## 🚀 Quick Start

**Want to deploy in 5 minutes?** → See [QUICK_START.md](./QUICK_START.md)

**Need detailed setup instructions?** → See [DEPLOYMENT_GUIDE.md](./DEPLOYMENT_GUIDE.md)

**Want to contribute?** → See [CONTRIBUTING.md](./CONTRIBUTING.md)

## ✨ Features

- **Multi-Platform Monitoring**: Tracks mentions across Reddit, Stack Overflow, Hacker News, X.com, LinkedIn, YouTube, Medium, and more
- **Context-Aware Filtering**: Uses NLP to ensure mentions are about the actual AKS service and not typos or unrelated acronyms  
- **Configurable Reporting**: Send daily or weekly reports via Microsoft Teams and email
- **Cloud-Native Architecture**: Built with Go and designed for Azure AKS deployment
- **Sentiment Analysis**: Identifies positive, negative, and neutral mentions
- **Real-time Alerts**: Immediate notifications for critical issues or complaints

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Data Sources  │    │   Processing    │    │   Notifications │
│                 │    │                 │    │                 │
│ • Reddit API    │───▶│ • Go Services   │───▶│ • Teams Webhook │
│ • Stack         │    │ • Context AI    │    │ • Email SMTP    │
│   Overflow      │    │ • Sentiment     │    │ • Azure Storage │
│ • Hacker News   │    │   Analysis      │    │                 │
│ • X.com API     │    │ • Data Storage  │    │                 │
│ • LinkedIn      │    │                 │    │                 │
│ • YouTube       │    │                 │    │                 │
│ • Medium        │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🎯 Monitored Platforms

### API-Required Sources
- **Reddit**: Subreddit posts and comments via Reddit API
- **Twitter/X**: Tweets and mentions via Twitter API v2  
- **YouTube**: Video titles, descriptions, and comments via YouTube Data API

### Public API Sources (No Keys Required)
- **Stack Overflow**: Questions and answers via public API
- **Hacker News**: Stories and comments via public API
- **Medium**: Articles and posts via RSS feeds
- **LinkedIn**: Public content via search APIs

## 🔧 Technology Stack

- **Language**: Go 1.21+
- **Deployment**: Azure Kubernetes Service (AKS)
- **Infrastructure**: Azure Bicep templates
- **Storage**: Azure Blob Storage
- **Identity**: Azure Workload Identity
- **Container Registry**: Azure Container Registry
- **Monitoring**: Azure Application Insights

## Quick Start

1. Clone this repository
2. Set up Azure resources using the provided Bicep templates
3. Configure environment variables
4. Deploy using Azure Container Apps

## Configuration

All configuration is done through environment variables:

### Required Settings

- `TEAMS_WEBHOOK_URL`: Microsoft Teams webhook URL (or use email)
- `NOTIFICATION_EMAIL`: Email address to send reports to (or use Teams)
- `AZURE_STORAGE_ACCOUNT`: Azure Storage account name for data persistence

### Optional Settings

- `REPORT_SCHEDULE`: "daily" or "weekly" (default: weekly)
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`: Email configuration (required if using email notifications)

### API Keys (Optional - sources are disabled if not provided)

## 💻 Local Development

```bash
# Clone repository
git clone <your-repository-url>
cd aks-mentions-bot

# Copy environment template
cp .env.example .env
# Edit .env with your API keys and webhook URLs

# Run tests
make test

# Test API connectivity  
make test-apis

# Run locally
make run
```

## 📊 What to Expect

### Reports Include
- **Total mentions** found across all sources
- **Breakdown by source** (Reddit, Twitter, YouTube, etc.)
- **Sentiment analysis** (positive, negative, neutral)  
- **Top sources** with most mentions
- **Sample mentions** with titles and links

### Default Behavior
- **Runs**: Every Monday 9 AM UTC (configurable)
- **Keywords**: "AKS", "Azure Kubernetes Service", "KubeFleet", "KAITO"
- **Context Filtering**: Filters out weapon-related AKS mentions
- **Storage**: All data saved to Azure Blob Storage

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
