package da

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethstorage/da-server/pkg/da/client"
)

type Config struct {
	SequencerIP  string
	ListenAddr   string
	StorePath    string
	SignerPKPath string
}

type Server struct {
	sync.WaitGroup
	config     *Config
	httpServer *http.Server
	listener   net.Listener
	store      *FileStore
	pk         *ecdsa.PrivateKey
}

func NewServer(config *Config) *Server {
	return &Server{
		config: config,
		httpServer: &http.Server{
			Addr: config.ListenAddr,
		},
		store: NewFileStore(config.StorePath),
	}
}

func (s *Server) Start(ctx context.Context) (err error) {

	pkBytes, err := os.ReadFile(s.config.SignerPKPath)
	if err != nil {
		return
	}
	pk, err := crypto.ToECDSA(common.FromHex(string(pkBytes)))
	if err != nil {
		return
	}
	s.pk = pk

	signer := crypto.PubkeyToAddress(pk.PublicKey)
	fmt.Println("signer", signer)

	mux := http.NewServeMux()

	mux.HandleFunc("/get/", s.handleGet)
	mux.HandleFunc("/put/", s.handlePut)
	mux.HandleFunc("/daproof", s.handleDAProof)
	s.httpServer.Handler = mux

	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

	s.Add(1)
	go func() {
		defer s.Done()
		err := s.httpServer.Serve(s.listener)
		if err != nil {
			fmt.Println("Serve error:", err)
		}
	}()

	return
}

func (s *Server) Stop(ctx context.Context) (err error) {

	err = s.httpServer.Shutdown(ctx)
	return
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := path.Base(r.URL.Path)
	comm, err := hexutil.Decode(key)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	input, err := s.store.Get(r.Context(), comm)
	if err != nil && errors.Is(err, client.ErrNotFound) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(input); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(r.RemoteAddr, s.config.SequencerIP) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	input, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := path.Base(r.URL.Path)
	comm, err := hexutil.Decode(key)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.store.Put(r.Context(), comm, input); err != nil {
		fmt.Println("store.Put", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(comm); err != nil {
		fmt.Println("w.Write", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleDAProof(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.RemoteAddr, s.config.SequencerIP) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	input, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var keys []common.Address
	err = json.Unmarshal(input, &keys)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, key := range keys {
		exist, err := s.store.Exist(key[:])
		if err != nil {
			fmt.Println("store.Exist", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !exist {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// all exist, sign da proof
	md := sha256.New()
	md.Write(input)
	hash := md.Sum(nil)

	sig, err := crypto.Sign(hash, s.pk)
	if err != nil {
		fmt.Println("crypto.Sign", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(sig); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
