package da

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethstorage/da-server/pkg/da/client"
)

type Config struct {
	SequencerIP string
	ListenAddr  string
	StorePath   string
	Relay       Relay
}

const DefaultRetry = 5

type Relay struct {
	Nodes []string
	Retry int
}
type Server struct {
	sync.WaitGroup
	config     *Config
	httpServer *http.Server
	listener   net.Listener
	store      *FileStore
	client     *client.Client
}

func NewServer(config *Config) *Server {
	s := &Server{
		config: config,
		httpServer: &http.Server{
			Addr: config.ListenAddr,
		},
		store: NewFileStore(config.StorePath),
	}

	if len(config.Relay.Nodes) > 0 {
		s.client = client.New(config.Relay.Nodes)
	}
	return s
}

func (s *Server) Start(ctx context.Context) (err error) {

	mux := http.NewServeMux()

	mux.HandleFunc("/get/", s.handleGet)
	mux.HandleFunc("/put/", s.handlePut)
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

	if s.client != nil {
		go func() {
			s.syncBlobToRelays(common.BytesToHash(comm), hexutil.Bytes(input))
		}()
	}
}

func (s *Server) syncBlobToRelays(comm common.Hash, blob hexutil.Bytes) {
	retry := s.config.Relay.Retry
	if retry <= 0 {
		retry = DefaultRetry
	}

	for i := 0; i < retry; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		err := s.client.SyncBlob(ctx, comm, blob)
		if err != nil {
			fmt.Printf("SyncBlob failed, comm:%s i: %d\n", comm.Hex(), i)
		} else {
			fmt.Printf("blob sync successfully, comm:%s\n", comm.Hex())
			return
		}
	}
	fmt.Printf("failed to sync, comm:%s\n", comm.Hex())
}
