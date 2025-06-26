package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/sirupsen/logrus"
)

// AzureStorage handles storing data in Azure Blob Storage
type AzureStorage struct {
	client        *azblob.Client
	containerName string
}

// Ensure AzureStorage implements StorageInterface
var _ StorageInterface = (*AzureStorage)(nil)

// NewAzureStorage creates a new Azure Storage client using managed identity
func NewAzureStorage(accountName, containerName string) (*AzureStorage, error) {
	if accountName == "" {
		return nil, fmt.Errorf("storage account name is required")
	}

	// Use managed identity for authentication (following Azure best practices)
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)
	client, err := azblob.NewClient(serviceURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure blob client: %w", err)
	}

	storage := &AzureStorage{
		client:        client,
		containerName: containerName,
	}

	// Ensure container exists
	if err := storage.ensureContainer(); err != nil {
		return nil, fmt.Errorf("failed to ensure container exists: %w", err)
	}

	return storage, nil
}

func (s *AzureStorage) ensureContainer() error {
	ctx := context.Background()
	
	// Try to create the container (this will fail if it already exists, which is fine)
	_, err := s.client.CreateContainer(ctx, s.containerName, nil)
	if err != nil {
		// Check if the error is because container already exists
		if !strings.Contains(err.Error(), "ContainerAlreadyExists") {
			return fmt.Errorf("failed to create container: %w", err)
		}
		logrus.Debugf("Container %s already exists", s.containerName)
	} else {
		logrus.Infof("Created container %s", s.containerName)
	}

	return nil
}

// Store saves data to Azure Blob Storage
func (s *AzureStorage) Store(filename string, data []byte) error {
	ctx := context.Background()

	// Upload the blob
	_, err := s.client.UploadBuffer(ctx, s.containerName, filename, data, &azblob.UploadBufferOptions{
		BlockSize:   int64(1024 * 1024), // 1MB blocks
		Concurrency: 3,
	})

	if err != nil {
		return fmt.Errorf("failed to upload blob %s: %w", filename, err)
	}

	logrus.Infof("Successfully stored %s in Azure Blob Storage", filename)
	return nil
}

// Retrieve gets data from Azure Blob Storage
func (s *AzureStorage) Retrieve(filename string) ([]byte, error) {
	ctx := context.Background()

	// Download the blob
	response, err := s.client.DownloadStream(ctx, s.containerName, filename, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download blob %s: %w", filename, err)
	}
	defer response.Body.Close()

	// Read the content
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read blob content: %w", err)
	}

	return data, nil
}

// List returns a list of blobs in the container
func (s *AzureStorage) List(prefix string) ([]string, error) {
	ctx := context.Background()

	var blobNames []string
	pager := s.client.NewListBlobsFlatPager(s.containerName, &azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs: %w", err)
		}

		for _, blob := range page.Segment.BlobItems {
			if blob.Name != nil {
				blobNames = append(blobNames, *blob.Name)
			}
		}
	}

	return blobNames, nil
}

// Delete removes a blob from Azure Blob Storage
func (s *AzureStorage) Delete(filename string) error {
	ctx := context.Background()

	_, err := s.client.DeleteBlob(ctx, s.containerName, filename, nil)
	if err != nil {
		return fmt.Errorf("failed to delete blob %s: %w", filename, err)
	}

	logrus.Infof("Successfully deleted %s from Azure Blob Storage", filename)
	return nil
}
