package tcp

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/myeof/gotcp/pkg/logger"
	"golang.org/x/time/rate"
)

type Server struct {
	listener *net.TCPListener
	rate     *rate.Limiter

	connBase
}

func NewServer() *Server {
	s := &Server{}
	s.showLog = true
	s.state = StateInit
	s.hbInterval = 10 * time.Second
	s.wg = sync.WaitGroup{}
	s.SetWorker(runtime.NumCPU() * 10)
	return s
}

func ListenAndServe(addr string, router *Router) error {
	s := NewServer()
	listen, err := s.Listen(addr)
	if err != nil {
		return err
	}
	log.Println("listen", listen.Addr().String())
	return s.Serve(router)
}

func (s *Server) SetCPS(n int) {
	if n > 0 {
		s.rate = rate.NewLimiter(rate.Limit(n), n)
	}
}

func (s *Server) Shutdown() {
	s.stopChan <- errors.New("shutdown")
}

func (s *Server) Listen(addr string) (*net.TCPListener, error) {
	var err error
	s.addr, err = net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	s.listener, err = net.ListenTCP("tcp", s.addr)
	if err != nil {
		return nil, err
	}
	return s.listener, nil
}

func (s *Server) Serve(router *Router) error {
	if s.listener == nil {
		return errors.New("listener is nil")
	}
	if s.state == StateRunning {
		return errors.New("server is running")
	}
	s.state = StateRunning
	defer func() {
		s.worker.Shutdown()
		s.wg.Wait()
		s.listener.Close()
		s.listener = nil
		s.state = StateTerminate
	}()

	s.router = router
	s.stopChan = make(chan error)
	defer close(s.stopChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.handleSignals(ctx)

	var exited bool
	go func() {
		for {
			if s.rate != nil {
				_ = s.rate.Wait(ctx)
			}
			conn, err := s.listener.AcceptTCP()
			if err != nil {
				if exited {
					return
				}
				s.stopChan <- err
				return
			}
			err = s.configureConnection(conn)
			if err != nil {
				logger.Errorw("Configure connection error", "error", err)
				_ = conn.Close()
				continue
			}
			session := NewSession(conn)
			go s.readHandler(ctx, session)
			go s.onConnected(NewContext(session, nil))
		}
	}()

	err := <-s.stopChan
	exited = true
	s.beforeShutdown()
	return err
}

func (s *Server) readHandler(ctx context.Context, session *Session) {
	s.wg.Add(1)
	defer s.wg.Done()
	exitChan := make(chan struct{})
	go func() {
		var err error
		for {
			var msg *Message
			msg, err = ReadMsg(session)
			if errors.Is(err, io.EOF) {
				// 对端关闭了连接
				break
			}
			if errors.Is(err, net.ErrClosed) {
				// 本端关闭了连接
				break
			}
			if err != nil {
				break
			}
			s.onMessage(session, msg)
		}
		s.onDisconnected(session, err)
		close(exitChan)
	}()
	select {
	case <-ctx.Done():
		return
	case <-exitChan:
		return
	}
}
