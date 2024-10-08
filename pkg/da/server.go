package da

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethstorage/da-server/pkg/da/client"
)

type Config struct {
	SequencerIP string
	ListenAddr  string
	StorePath   string
	ExpireHours int64
}

type Server struct {
	sync.WaitGroup
	config     *Config
	httpServer *http.Server
	listener   net.Listener
	store      *FileStore
	cancel     context.CancelFunc
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

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	if s.config.ExpireHours > 0 {
		s.Add(1)
		go func() {
			defer s.Done()
			s.expireData(ctx)
		}()
	}

	return
}

func (s *Server) expireData(ctx context.Context) {
	if s.config.ExpireHours <= 0 {
		panic("expireData should only be called when ExpireHours>0")
	}

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// alternatively, we can use such command for manual operation on linux to prune files older than 7 days:
			// 	find target_dir -type f -mmin +10080 -exec rm {} +
			err := filepath.Walk(s.config.StorePath, func(path string, info fs.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}

				if time.Since(info.ModTime()) > time.Duration(s.config.ExpireHours)*time.Hour {
					fmt.Printf("deleting file %s, mod time: %v\n", path, info.ModTime())
					err := os.Remove(path)
					if err != nil {
						fmt.Printf("failed to delete file %s, error: %v\n", path, err)
					}
				}
				return nil
			})
			if err != nil {
				fmt.Printf("filepath.Walk failed, error: %v\n", err)
			}
		}
	}
}

func (s *Server) Stop(ctx context.Context) (err error) {
	s.cancel()
	err = s.httpServer.Shutdown(ctx)
	if err != nil {
		return
	}
	s.Wait()
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
