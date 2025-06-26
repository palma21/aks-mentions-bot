#!/bin/bash

echo "🚀 AKS Mentions Bot - Validation Script"
echo "======================================"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed"
    echo "Please install Go 1.21 or later from https://golang.org/dl/"
    exit 1
fi

echo "✅ Go is installed: $(go version)"

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.21"

if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" = "$REQUIRED_VERSION" ]; then
    echo "✅ Go version is compatible"
else
    echo "❌ Go version $GO_VERSION is too old. Required: $REQUIRED_VERSION"
    exit 1
fi

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "❌ go.mod file not found. Please run this script from the project root."
    exit 1
fi

echo "✅ Found go.mod file"

# Download dependencies
echo "📦 Downloading dependencies..."
if go mod tidy; then
    echo "✅ Dependencies downloaded successfully"
else
    echo "❌ Failed to download dependencies"
    exit 1
fi

# Check if main.go exists
if [ ! -f "cmd/bot/main.go" ]; then
    echo "❌ Main application file not found"
    exit 1
fi

echo "✅ Main application file found"

# Try to build the application
echo "🔨 Building application..."
if go build -o bin/aks-mentions-bot ./cmd/bot; then
    echo "✅ Application built successfully"
else
    echo "❌ Build failed"
    exit 1
fi

# Run tests
echo "🧪 Running tests..."
if go test ./...; then
    echo "✅ All tests passed"
else
    echo "⚠️ Some tests failed"
fi

# Check environment file
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        echo "📋 Creating .env file from example..."
        cp .env.example .env
        echo "✅ .env file created. Please edit it with your configuration."
    else
        echo "⚠️ No .env file found. You'll need to create one for configuration."
    fi
else
    echo "✅ .env file exists"
fi

# Check Docker
if command -v docker &> /dev/null; then
    echo "✅ Docker is available"
    echo "🐳 You can build the Docker image with: docker build -t aks-mentions-bot ."
else
    echo "⚠️ Docker not found. Install Docker to build container images."
fi

# Check Azure CLI
if command -v az &> /dev/null; then
    echo "✅ Azure CLI is available"
    echo "☁️ You can deploy with: azd up"
else
    echo "⚠️ Azure CLI not found. Install Azure CLI for deployment."
fi

echo ""
echo "🎉 Validation complete!"
echo ""
echo "Next steps:"
echo "1. Edit .env file with your API keys and configuration"
echo "2. Test locally: go run cmd/bot/main.go"
echo "3. Deploy to Azure: azd up"
echo ""
echo "For more information, see DEPLOYMENT_GUIDE.md"
