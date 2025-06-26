#!/bin/bash
set -e

# Setup Workload Identity for AKS Mentions Bot
# This script configures the federated identity credential needed for workload identity

echo "🔧 Setting up Workload Identity for AKS Mentions Bot..."

# Get values from azd environment
RESOURCE_GROUP=$(azd env get-value AZURE_RESOURCE_GROUP)
AKS_CLUSTER_NAME=$(azd env get-value AKS_CLUSTER_NAME)
WORKLOAD_IDENTITY_CLIENT_ID=$(azd env get-value WORKLOAD_IDENTITY_CLIENT_ID)

if [ -z "$RESOURCE_GROUP" ] || [ -z "$AKS_CLUSTER_NAME" ] || [ -z "$WORKLOAD_IDENTITY_CLIENT_ID" ]; then
    echo "❌ Error: Missing required environment variables. Make sure azd provision has been run successfully."
    echo "Required: AZURE_RESOURCE_GROUP, AKS_CLUSTER_NAME, WORKLOAD_IDENTITY_CLIENT_ID"
    exit 1
fi

# Get AKS OIDC issuer URL
echo "📡 Getting AKS OIDC issuer URL..."
AKS_OIDC_ISSUER=$(az aks show --name $AKS_CLUSTER_NAME --resource-group $RESOURCE_GROUP --query "oidcIssuerProfile.issuerUrl" -o tsv)

if [ -z "$AKS_OIDC_ISSUER" ]; then
    echo "❌ Error: Could not retrieve OIDC issuer URL. Make sure the AKS cluster has OIDC issuer enabled."
    exit 1
fi

echo "🔑 OIDC Issuer: $AKS_OIDC_ISSUER"

# Create federated identity credential
echo "🔗 Creating federated identity credential..."
az identity federated-credential create \
    --name "aks-mentions-bot-federated" \
    --identity-name "$(az identity list --resource-group $RESOURCE_GROUP --query "[0].name" -o tsv)" \
    --resource-group $RESOURCE_GROUP \
    --issuer $AKS_OIDC_ISSUER \
    --subject "system:serviceaccount:aks-mentions-bot:aks-mentions-bot-sa" \
    --audience "api://AzureADTokenExchange"

echo "✅ Workload Identity setup completed successfully!"
echo ""
echo "📋 Summary:"
echo "   Resource Group: $RESOURCE_GROUP"
echo "   AKS Cluster: $AKS_CLUSTER_NAME"
echo "   OIDC Issuer: $AKS_OIDC_ISSUER"
echo "   Service Account: system:serviceaccount:aks-mentions-bot:aks-mentions-bot-sa"
echo "   Client ID: $WORKLOAD_IDENTITY_CLIENT_ID"
echo ""
echo "🚀 You can now deploy the application to AKS:"
echo "   kubectl apply -f k8s/deployment.yaml"
