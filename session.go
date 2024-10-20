package tcp

import (
	"net"
	"sync"
)

type Session struct {
	conn *net.TCPConn

	closeChan chan error

	sync.RWMutex
}

func NewSession(conn *net.TCPConn) *Session {
	return &Session{
		conn:      conn,
		closeChan: make(chan error, 1),
	}
}

func (s *Session) Conn() *net.TCPConn {
	return s.conn
}

func (s *Session) Remote() string {
	return s.conn.RemoteAddr().String()
}

func (s *Session) Close() error {
	s.Lock()
	defer s.Unlock()
	if s.conn != nil {
		err := s.conn.Close()
		return err
	}
	return nil
}
