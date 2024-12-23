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
	"golang.org/x/sync/errgroup"
)

type Client struct {
	urls []string
}

func New(urls []string) *Client {
	if len(urls) == 0 {
		panic("empty urls")
	}
	return &Client{urls: urls}
}

const blobSize = 128 * 1024

// UploadBlobs is for sequencer to upload Blob.
func (c *Client) UploadBlobs(ctx context.Context, envelope *eth.ExecutionPayloadEnvelope) error {

	if len(envelope.BlobsBundle.Commitments) != len(envelope.BlobsBundle.Blobs) {
		return fmt.Errorf("invvalid envelope")
	}

	for i := range envelope.BlobsBundle.Blobs {
		blob := envelope.BlobsBundle.Blobs[i]
		if len(blob) != blobSize {
			return fmt.Errorf("invalid blob size:%d, index:%d", len(blob), i)
		}
		blobHash := eth.KZGToVersionedHash(kzg4844.Commitment(envelope.BlobsBundle.Commitments[i]))

		err := c.uploadBlob(ctx, blob, blobHash)
		if err != nil {
			return err
		}

	}

	return nil
}

// SyncBlob is to sync Blobs to relays.
func (c *Client) SyncBlob(ctx context.Context, comm common.Hash, blob hexutil.Bytes) error {
	if len(blob) != blobSize {
		return fmt.Errorf("invalid blob size:%d", len(blob))
	}
	return c.uploadBlob(ctx, blob, comm)
}

func (c *Client) uploadBlob(ctx context.Context, blob hexutil.Bytes, blobHash common.Hash) error {
	if len(c.urls) == 1 {
		err := c.uploadBlobTo(ctx, blob, blobHash, 0)
		return err
	}

	// when there're multiple urls, upload blobs concurrently.
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < len(c.urls); i++ {
		i := i
		g.Go(func() error {
			return c.uploadBlobTo(ctx, blob, blobHash, i)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func (c *Client) uploadBlobTo(ctx context.Context, blob hexutil.Bytes, blobHash common.Hash, i int) error {
	key := blobHash.Hex()
	body := bytes.NewReader(blob)
	url := fmt.Sprintf("%s/put/%s", c.urls[i], key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
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
	return nil
}

// ErrNotFound is returned when the server could not find the input.
var ErrNotFound = errors.New("not found")

func (c *Client) GetBlobs(ctx context.Context, blobHashes []common.Hash) (blobs []hexutil.Bytes, err error) {
	return c.GetBlobsFrom(ctx, blobHashes, 0)
}

func (c *Client) GetBlobsFrom(ctx context.Context, blobHashes []common.Hash, idx int) (blobs []hexutil.Bytes, err error) {
	if idx >= len(c.urls) {
		return nil, fmt.Errorf("index out of range")
	}
	for i, blobHash := range blobHashes {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/get/%s", c.urls[idx], blobHash.Hex()), nil)
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
		commit, err := kzg4844.BlobToCommitment(&fixedBlob)
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
