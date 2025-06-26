package storage

// StorageInterface defines the contract for storage operations
type StorageInterface interface {
	Store(filename string, data []byte) error
	Retrieve(filename string) ([]byte, error)
	List(prefix string) ([]string, error)
	Delete(filename string) error
}
