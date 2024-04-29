package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type Client struct {
	url    string
	signer common.Address
}

func New(url string, signer common.Address) *Client {
	return &Client{url: url, signer: signer}
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
		url := fmt.Sprintf("%s/put/0x%s", c.url, key)
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
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("%s/get/0x%s", c.url, blobHash.Hex()), nil)
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

func (c *Client) CheckDAProof(blobHashes []common.Hash, daProof []byte) error {
	if (len(blobHashes) == 0) == (len(daProof) != 0) {
		return fmt.Errorf("blobHashes and daProof should have the same emptiness")
	}
	if len(blobHashes) == 0 {
		return nil
	}

	jsonPayload, err := json.Marshal(blobHashes)
	if err != nil {
		return err
	}

	md := sha256.New()
	md.Write(jsonPayload)
	hash := md.Sum(nil)
	pubkey, err := crypto.Ecrecover(hash, daProof)
	if err != nil {
		return err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])
	if signer != c.signer {
		return fmt.Errorf("sign not match, expect:%v got:%v", c.signer, signer)
	}
	return nil
}

func (c *Client) DAProof(blobHashes []common.Hash) (proof []byte, err error) {

	jsonPayload, err := json.Marshal(blobHashes)
	if err != nil {
		return
	}
	url := fmt.Sprintf("%s/daproof", c.url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get preimage: %v", resp.StatusCode)
	}
	defer resp.Body.Close()
	proof, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	md := sha256.New()
	md.Write(jsonPayload)
	hash := md.Sum(nil)
	pubkey, err := crypto.Ecrecover(hash, proof)
	if err != nil {
		return nil, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])
	if signer != c.signer {
		return nil, fmt.Errorf("sign not match, expect:%v got:%v", c.signer, signer)
	}

	return
}
