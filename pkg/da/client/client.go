package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type Client struct {
	url string
}

func New(url string) *Client {
	return &Client{url: url}
}

const blobSize = 128 * 1024

func (c *Client) UploadBlobs(envelope *eth.ExecutionPayloadEnvelope) error {

	if len(envelope.BlobsBundle.Commitments) != len(envelope.BlobsBundle.Blobs) {
		return fmt.Errorf("invvalid envelope")
	}

	for i := range envelope.BlobsBundle.Blobs {
		blob := envelope.BlobsBundle.Blobs[i]
		if len(blob) != blobSize {
			return fmt.Errorf("invalid blob size:%d, index:%d", len(blob), i)
		}
		blobHash := eth.KZGToVersionedHash(kzg4844.Commitment(envelope.BlobsBundle.Commitments[i]))

		key := blobHash.Hex()
		body := bytes.NewReader(blob)
		url := fmt.Sprintf("%s/put/0x%x", c.url, key)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, body)
		if err != nil {
			return fmt.Errorf("NewRequestWithContext failed:%v", err)
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to store preimage: %v", resp.StatusCode)
		}
	}

	return nil
}

// ErrNotFound is returned when the server could not find the input.
var ErrNotFound = errors.New("not found")

func (c *Client) GetBlobs(blobHashes []common.Hash) (blobs []hexutil.Bytes, err error) {
	for i, blobHash := range blobHashes {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("%s/get/0x%x", c.url, blobHash.Hex()), nil)
		if err != nil {
			return nil, fmt.Errorf("NewRequestWithContext failed: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("blob not found for %s: %w", blobHash, ErrNotFound)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get preimage: %v", resp.StatusCode)
		}
		defer resp.Body.Close()
		blob, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if len(blob) != blobSize {
			return nil, fmt.Errorf("invalid blob size:%d, index:%d", len(blob), i)
		}

		var fixedBlob kzg4844.Blob
		copy(fixedBlob[:], blob)
		commit, err := kzg4844.BlobToCommitment(fixedBlob)
		if err != nil {
			return nil, fmt.Errorf("BlobToCommitment failed:%w, index:%d", err, i)
		}
		if eth.KZGToVersionedHash(commit) != blobHash {
			return nil, fmt.Errorf("invalid blob for %s", blobHash)
		}
		blobs = append(blobs, blob)
	}

	return
}
