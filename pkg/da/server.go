package da

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Config struct {
	Sequencer  common.Address
	ListenAddr string
	StorePath  string
}

type Server struct {
	sync.WaitGroup
	config     *Config
	httpServer *http.Server
	listener   net.Listener
	store      *FileStore
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
	if err != nil && errors.Is(err, ErrNotFound) {
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(comm); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
