package da

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path"
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
		panic("expireData should only be called when ExpireDays>0")
	}

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	bashCmd := fmt.Sprintf("find %s -type f -mmin +%d -exec rm {} +", s.config.StorePath, s.config.ExpireHours*60)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cmd := exec.Command("bash", "-c", bashCmd)
			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("failed to prune expired files, err: %v, detail:%s\n", err, string(output))
			} else {
				fmt.Printf("prune ran successfully at %v\n", time.Now())
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
