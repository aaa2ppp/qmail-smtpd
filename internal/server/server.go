package server

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"qmail-smtpd/internal/logger"
)

type ConnectionHandler interface {
	Handle(ctx context.Context, conn net.Conn) error
}

type Config struct {
	MaxConnections  int    // -c <num>
	OverloadMessage []byte // например "421 server too busy\r\n"
	WriteTimeout    time.Duration
}

type Server struct {
	cfg      Config
	handler  ConnectionHandler
	listener net.Listener
	wg       sync.WaitGroup
}

func New(cfg Config, handler ConnectionHandler) *Server {
	return &Server{
		cfg:     cfg,
		handler: handler,
	}
}

func (srv *Server) logger() *slog.Logger {
	return slog.Default()
}

func (srv *Server) Serve(listener net.Listener) error {
	log := srv.logger().With("op", "Server.Serve")

	var sem semaphore
	if n := srv.cfg.MaxConnections; n > 0 {
		sem = makeSemaphore(n)
	}

	srv.listener = listener
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		if !sem.acquire() {
			if srv.cfg.OverloadMessage != nil {
				if timeout := srv.cfg.WriteTimeout; timeout > 0 {
					conn.SetWriteDeadline(time.Now().Add(timeout))
				}
				conn.Write(srv.cfg.OverloadMessage)
			}
			conn.Close()
			continue
		}

		srv.wg.Add(1)
		go func(conn net.Conn) {
			cid := rand.Int64()
			log := log.With("cid", cid)

			defer func() {
				if p := recover(); p != nil {
					log.Error("panic recovered", "panic", p, "stack", string(debug.Stack()))
				}
				conn.Close()
				sem.release()
				srv.wg.Done()
			}()

			ctx := logger.Context(ctx, srv.logger().With("cid", cid))

			if err := srv.handler.Handle(ctx, conn); err != nil {
				log.Error("handle connection failed", "error", err)
			}
		}(conn)
	}
}

func (srv *Server) Close() error {
	return srv.listener.Close()
}

func (srv *Server) Wait(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		srv.wg.Wait()
		close(done)
	}()

	tm := time.NewTimer(timeout)
	select {
	case <-tm.C:
		return errors.New("timeout expired")
	case <-done:
		tm.Stop()
	}

	return nil
}
