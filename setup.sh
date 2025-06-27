#!/bin/bash

# AKS Mentions Bot Setup Script
# This script helps you set up the project from templates

set -e

echo "🚀 Setting up AKS Mentions Bot from templates..."

# Check if we're in the right directory
if [ ! -f "azure.yaml" ]; then
    echo "❌ Error: azure.yaml not found. Please run this script from the project root directory."
    exit 1
fi

# Copy environment template
if [ ! -f ".env" ]; then
    echo "📝 Copying .env.example to .env..."
    cp .env.example .env
    echo "✅ Created .env - please edit with your API keys"
else
    echo "ℹ️  .env already exists, skipping..."
fi

# Copy Kubernetes templates
if [ ! -f "k8s/deployment.local.yaml" ]; then
    echo "📝 Copying deployment template..."
    cp k8s/deployment.template.yaml k8s/deployment.local.yaml
    echo "✅ Created k8s/deployment.local.yaml - please edit with your values"
else
    echo "ℹ️  k8s/deployment.local.yaml already exists, skipping..."
fi

if [ ! -f "k8s/secrets.local.yaml" ]; then
    echo "📝 Copying secrets template..."
    cp k8s/secrets.template.yaml k8s/secrets.local.yaml
    echo "✅ Created k8s/secrets.local.yaml - please edit with your values"
else
    echo "ℹ️  k8s/secrets.local.yaml already exists, skipping..."
fi

# Copy infrastructure template
if [ ! -f "infra/main.parameters.local.json" ]; then
    echo "📝 Copying infrastructure parameters template..."
    cp infra/main.parameters.template.json infra/main.parameters.local.json
    echo "✅ Created infra/main.parameters.local.json"
else
    echo "ℹ️  infra/main.parameters.local.json already exists, skipping..."
fi

echo ""
echo "🎉 Setup complete! Next steps:"
echo ""
echo "1. Edit .env with your API keys and configuration"
echo "2. Edit k8s/deployment.local.yaml with your Azure resource names"
echo "3. Edit k8s/secrets.local.yaml with your Key Vault and identity details"
echo "4. Run 'azd up' to deploy everything to Azure"
echo ""
echo "📚 See README.md for detailed configuration instructions"
echo ""
echo "🔒 Note: All .local.* files are gitignored to protect your secrets"
echo "💡 The 'azd up' command will automatically:"
echo "   - Deploy infrastructure (AKS, ACR, Key Vault, etc.)"
echo "   - Build and push your container image"
echo "   - Deploy to Kubernetes using your .local.yaml files"
