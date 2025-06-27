#!/bin/bash
# AKS Mentions Bot - Key Vault Secrets Management Script
# This script helps you create and manage secrets in Azure Key Vault

set -e

echo "🔐 AKS Mentions Bot - Key Vault Secrets Management"
echo "=================================================="

# Check if azd is available
if ! command -v azd &> /dev/null; then
    echo "❌ Azure Developer CLI (azd) is required but not installed."
    echo "   Install it from: https://learn.microsoft.com/en-us/azure/developer/azure-developer-cli/install-azd"
    exit 1
fi

# Check if az is available
if ! command -v az &> /dev/null; then
    echo "❌ Azure CLI (az) is required but not installed."
    echo "   Install it from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
    exit 1
fi

# Get Key Vault name from azd environment
echo "🔍 Getting Key Vault information from azd environment..."
KEYVAULT_NAME=$(azd env get-value AZURE_KEY_VAULT_NAME 2>/dev/null || echo "")

if [ -z "$KEYVAULT_NAME" ]; then
    echo "❌ Could not get Key Vault name from azd environment."
    echo "   Make sure you have run 'azd provision' first."
    exit 1
fi

echo "✅ Key Vault: $KEYVAULT_NAME"

# Function to create a secret
create_secret() {
    local secret_name=$1
    local description=$2
    local example=$3
    
    echo ""
    echo "📝 Creating secret: $secret_name"
    echo "   Description: $description"
    echo "   Example: $example"
    echo "   ⚠️  Note: Input will be visible on screen"
    
    # Check if secret already exists
    if az keyvault secret show --vault-name "$KEYVAULT_NAME" --name "$secret_name" &>/dev/null; then
        echo "⚠️  Secret '$secret_name' already exists."
        read -p "   Do you want to update it? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "   Skipping $secret_name"
            return
        fi
    fi
    
    read -p "   Enter value for $secret_name: " secret_value
    echo
    
    if [ -z "$secret_value" ]; then
        echo "   ⚠️  Empty value, skipping $secret_name"
        return
    fi
    
    if az keyvault secret set --vault-name "$KEYVAULT_NAME" --name "$secret_name" --value "$secret_value" 2>/dev/null; then
        echo "   ✅ Secret '$secret_name' created successfully"
    else
        echo "   ❌ Failed to create secret '$secret_name'"
        echo "   Trying to get error details..."
        az keyvault secret set --vault-name "$KEYVAULT_NAME" --name "$secret_name" --value "$secret_value"
    fi
}

# Function to list all secrets
list_secrets() {
    echo ""
    echo "📋 Current secrets in Key Vault:"
    az keyvault secret list --vault-name "$KEYVAULT_NAME" --query "[].{Name:name,Enabled:attributes.enabled,Updated:attributes.updated}" -o table
}

# Function to test secret access
test_secret_access() {
    echo ""
    echo "🧪 Testing secret access..."
    
    local secrets=("teams-webhook-url" "reddit-client-id" "reddit-client-secret" "twitter-bearer-token" "youtube-api-key" "notification-email" "smtp-password")
    
    for secret in "${secrets[@]}"; do
        if az keyvault secret show --vault-name "$KEYVAULT_NAME" --name "$secret" --query "value" -o tsv &>/dev/null; then
            echo "   ✅ $secret - accessible"
        else
            echo "   ❌ $secret - not found or not accessible"
        fi
    done
}

# Main menu
while true; do
    echo ""
    echo "🎯 What would you like to do?"
    echo "1) Create all required secrets (interactive)"
    echo "2) List existing secrets"
    echo "3) Test secret access"
    echo "4) Create a specific secret"
    echo "5) Exit"
    echo ""
    read -p "Choose an option (1-5): " choice
    
    case $choice in
        1)
            echo ""
            echo "🚀 Creating all required secrets for AKS Mentions Bot..."
            echo "   You'll be prompted for each secret value."
            echo ""
            
            create_secret "teams-webhook-url" "Azure Logic Apps HTTP trigger URL" "https://prod-xx.westus2.logic.azure.com:443/workflows/..."
            create_secret "reddit-client-id" "Reddit API application ID" "your_reddit_app_id"
            create_secret "reddit-client-secret" "Reddit API secret" "your_reddit_secret"
            create_secret "twitter-bearer-token" "Twitter API v2 bearer token" "AAAAAAAAAAAAAAAAAAAAAxxxx"
            create_secret "youtube-api-key" "YouTube Data API v3 key" "AIzaSyXXXXXXXXXXXXXXXX"
            create_secret "notification-email" "Email for error notifications" "alerts@company.com"
            create_secret "smtp-password" "SMTP password for email" "your_smtp_password"
            
            echo ""
            echo "✅ Secret creation process completed!"
            ;;
        2)
            list_secrets
            ;;
        3)
            test_secret_access
            ;;
        4)
            echo ""
            read -p "Enter secret name: " secret_name
            read -p "Enter description: " description
            create_secret "$secret_name" "$description" "your_value"
            ;;
        5)
            echo ""
            echo "👋 Goodbye!"
            exit 0
            ;;
        *)
            echo "❌ Invalid option. Please choose 1-5."
            ;;
    esac
done
