package server

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/logger"
)

type ConnectionHandler interface {
	Handle(ctx context.Context, env env.Env, conn net.Conn) error
}

type Server struct {
	Env            env.Env
	MaxConnections int // -c <num>
	Handler        Handler
	wg             sync.WaitGroup
}

func (srv *Server) logger() *slog.Logger {
	return slog.Default()
}

func (srv *Server) Serve(listener net.Listener) error {
	log := srv.logger().With("op", "Server.Serve")
	senv := srv.Env
	if senv == nil {
		senv = env.New(os.Environ())
	}

	var sem semaphore
	if n := srv.MaxConnections; n > 0 {
		sem = makeSemaphore(n)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		if !sem.acquire() {
			conn.Close()
			continue
		}

		srv.wg.Add(1)
		go func(env env.Env, conn net.Conn) {
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

			if err := srv.Handler.Handle(ctx, env, conn); err != nil {
				log.Error("handle connection failed", "error", err)
			}
		}(senv.Clone(), conn)
	}
}

func (srv *Server) WaitAllConnections(timeout time.Duration) error {
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
