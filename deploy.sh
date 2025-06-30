#!/bin/bash

# AZD Deployment Script
# This script prepares the deployment files for azd up

set -e

echo "🚀 Preparing AKS Mentions Bot for AZD deployment..."

# Check if local files exist
if [ ! -f "k8s/deployment.local.yaml" ] || [ ! -f "k8s/secrets.local.yaml" ]; then
    echo "❌ Error: Local deployment files not found. Run setup.sh first."
    exit 1
fi

# Create temporary deployment directory for azd
echo "📁 Creating temporary deployment files for azd..."
mkdir -p k8s/deploy

# Copy local files to deploy directory
cp k8s/deployment.local.yaml k8s/deploy/deployment.yaml
cp k8s/secrets.local.yaml k8s/deploy/secrets.yaml

echo "✅ Deployment files prepared in k8s/deploy/"
echo "Now run: azd up"
echo ""
echo "🧹 After deployment, run: rm -rf k8s/deploy"
