@description('Primary location for all resources')
param location string = resourceGroup().location

@description('Resource token to make resource names unique')
param resourceToken string = toLower(uniqueString(subscription().id, resourceGroup().id, location))

@description('Tags to apply to all resources')
param tags object = {
  'azd-env-name': 'aks-mentions-bot'
}

@description('Admin username for AKS cluster nodes')
param adminUsername string = 'azureuser'

@description('SSH RSA public key file as a string for AKS cluster nodes')
param sshRSAPublicKey string

// Core resources
var abbrs = loadJsonContent('abbreviations.json')
var storageAccountName = 'st${resourceToken}mentions'
var containerRegistryName = 'cr${resourceToken}mentions'
var logAnalyticsName = '${abbrs.operationalInsightsWorkspaces}${resourceToken}'
var appInsightsName = '${abbrs.insightsComponents}${resourceToken}'
var keyVaultName = '${abbrs.keyVaultVaults}${resourceToken}'
var aksClusterName = 'aks-mentions-bot-${resourceToken}'
var vnetName = 'vnet-${resourceToken}'

// User-assigned managed identity for AKS cluster
resource managedIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: '${abbrs.managedIdentityUserAssignedIdentities}${resourceToken}'
  location: location
  tags: tags
}

// Virtual Network for AKS cluster
resource vnet 'Microsoft.Network/virtualNetworks@2023-09-01' = {
  name: vnetName
  location: location
  tags: tags
  properties: {
    addressSpace: {
      addressPrefixes: [
        '10.0.0.0/16'
      ]
    }
    subnets: [
      {
        name: 'aks-subnet'
        properties: {
          addressPrefix: '10.0.1.0/24'
        }
      }
    ]
  }
}

// Log Analytics workspace
resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2022-10-01' = {
  name: logAnalyticsName
  location: location
  tags: tags
  properties: {
    sku: {
      name: 'PerGB2018'
    }
    retentionInDays: 30
  }
}

// Application Insights
resource appInsights 'Microsoft.Insights/components@2020-02-02' = {
  name: appInsightsName
  location: location
  tags: tags
  kind: 'web'
  properties: {
    Application_Type: 'web'
    WorkspaceResourceId: logAnalytics.id
  }
}

// Container Registry
resource containerRegistry 'Microsoft.ContainerRegistry/registries@2023-07-01' = {
  name: containerRegistryName
  location: location
  tags: tags
  sku: {
    name: 'Basic'
  }
  properties: {
    adminUserEnabled: false
  }
}

// Storage Account for mentions data
resource storageAccount 'Microsoft.Storage/storageAccounts@2023-01-01' = {
  name: storageAccountName
  location: location
  tags: tags
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    encryption: {
      services: {
        blob: {
          enabled: true
        }
      }
      keySource: 'Microsoft.Storage'
    }
    supportsHttpsTrafficOnly: true
    minimumTlsVersion: 'TLS1_2'
  }
}

// Blob container for storing mentions
resource blobServices 'Microsoft.Storage/storageAccounts/blobServices@2023-01-01' = {
  parent: storageAccount
  name: 'default'
}

resource mentionsContainer 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-01-01' = {
  parent: blobServices
  name: 'mentions'
  properties: {
    publicAccess: 'None'
  }
}

// Key Vault for secrets
resource keyVault 'Microsoft.KeyVault/vaults@2023-07-01' = {
  name: keyVaultName
  location: location
  tags: tags
  properties: {
    sku: {
      family: 'A'
      name: 'standard'
    }
    tenantId: subscription().tenantId
    enableRbacAuthorization: true
    enableSoftDelete: true
    softDeleteRetentionInDays: 7
  }
}

// AKS Cluster
resource aksCluster 'Microsoft.ContainerService/managedClusters@2024-09-01' = {
  name: aksClusterName
  location: location
  tags: union(tags, {
    'azd-service-name': 'aks-cluster'
  })
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${managedIdentity.id}': {}
    }
  }
  properties: {
    dnsPrefix: 'aks-mentions-bot'
    kubernetesVersion: '1.29.9'
    enableRBAC: true
    
    // Node pools configuration
    agentPoolProfiles: [
      {
        name: 'systempool'
        count: 2
        vmSize: 'Standard_B2s'
        osType: 'Linux'
        mode: 'System'
        enableAutoScaling: true
        minCount: 1
        maxCount: 3
        vnetSubnetID: '${vnet.id}/subnets/aks-subnet'
        type: 'VirtualMachineScaleSets'
        osDiskSizeGB: 30
        maxPods: 30
      }
    ]
    
    // Linux profile for SSH access
    linuxProfile: {
      adminUsername: adminUsername
      ssh: {
        publicKeys: [
          {
            keyData: sshRSAPublicKey
          }
        ]
      }
    }
    
    // Network configuration
    networkProfile: {
      networkPlugin: 'azure'
      networkMode: 'transparent'
      serviceCidr: '10.1.0.0/16'
      dnsServiceIP: '10.1.0.10'
      loadBalancerSku: 'standard'
    }
    
    // Azure Monitor integration
    addonProfiles: {
      omsagent: {
        enabled: true
        config: {
          logAnalyticsWorkspaceResourceID: logAnalytics.id
        }
      }
      azureKeyvaultSecretsProvider: {
        enabled: true
      }
    }
    
    // Security profile
    securityProfile: {
      workloadIdentity: {
        enabled: true
      }
    }
    
    // OIDC Issuer for workload identity
    oidcIssuerProfile: {
      enabled: true
    }
    
    // Auto-upgrade configuration
    autoUpgradeProfile: {
      upgradeChannel: 'stable'
      nodeOSUpgradeChannel: 'NodeImage'
    }
  }
}

// Role assignments for the managed identity
resource storageDataContributor 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: storageAccount
  name: guid(subscription().id, managedIdentity.id, 'ba92f5b4-2d11-453d-a403-e96b0029c9fe')
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', 'ba92f5b4-2d11-453d-a403-e96b0029c9fe') // Storage Blob Data Contributor
    principalId: managedIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource acrPullRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: containerRegistry
  name: guid(subscription().id, managedIdentity.id, '7f951dda-4ed3-4680-a7ca-43fe172d538d')
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d') // AcrPull
    principalId: managedIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource keyVaultSecretsUser 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: keyVault
  name: guid(subscription().id, managedIdentity.id, '4633458b-17de-408a-b874-0445c86b69e6')
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '4633458b-17de-408a-b874-0445c86b69e6') // Key Vault Secrets User
    principalId: managedIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

// Additional role assignment for AKS to pull from ACR
resource aksAcrPullRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: containerRegistry
  name: guid(subscription().id, aksCluster.id, '7f951dda-4ed3-4680-a7ca-43fe172d538d')
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d') // AcrPull
    principalId: aksCluster.properties.identityProfile.kubeletidentity.objectId
    principalType: 'ServicePrincipal'
  }
}

// Outputs
output AZURE_LOCATION string = location
output AZURE_CONTAINER_REGISTRY_ENDPOINT string = containerRegistry.properties.loginServer
output AZURE_STORAGE_ACCOUNT_NAME string = storageAccount.name
output AZURE_KEY_VAULT_NAME string = keyVault.name
output AKS_CLUSTER_NAME string = aksCluster.name
output AKS_CLUSTER_FQDN string = aksCluster.properties.fqdn
output SERVICE_BOT_IDENTITY_PRINCIPAL_ID string = managedIdentity.properties.principalId
output WORKLOAD_IDENTITY_CLIENT_ID string = managedIdentity.properties.clientId
