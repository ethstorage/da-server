package da

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path"

	"github.com/ethstorage/da-server/pkg/da/client"
)

type FileStore struct {
	directory string
}

func NewFileStore(directory string) *FileStore {
	err := os.MkdirAll(directory, 0755)
	if err != nil {
		panic(fmt.Sprintf("failed to create directory:%s", directory))
	}

	return &FileStore{
		directory: directory,
	}
}

func (s *FileStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	data, err := os.ReadFile(s.fileName(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, client.ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

func (s *FileStore) Put(ctx context.Context, key []byte, value []byte) error {
	return os.WriteFile(s.fileName(key), value, 0600)
}

func (s *FileStore) Exist(key []byte) (exists bool, err error) {
	_, err = os.Stat(s.fileName(key))
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (s *FileStore) fileName(key []byte) string {
	return path.Join(s.directory, hex.EncodeToString(key))
}
