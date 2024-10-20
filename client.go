package tcp

import (
	"context"
	"net"
	"runtime"
	"sync"
	"time"
)

type Client struct {
	conn    *net.TCPConn
	session *Session

	connBase
}

func NewClient() *Client {
	c := &Client{}
	c.wg = sync.WaitGroup{}
	c.hbInterval = 10 * time.Second
	c.state = StateInit
	c.SetWorker(runtime.NumCPU() * 10)
	return c
}

func (c *Client) Connect(addr string, router *Router) error {
	var err error
	c.addr, err = net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}
	c.conn, err = net.DialTCP("tcp", nil, c.addr)
	if err != nil {
		return err
	}

	c.session = NewSession(c.conn)
	defer func() {
		_ = c.session.Close()
		c.session = nil
	}()

	c.state = StateRunning
	defer func() {
		c.wg.Wait()
		c.state = StateTerminate
	}()

	// 处理信号
	c.stopChan = make(chan error)
	defer close(c.stopChan)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.handleSignals(ctx)

	// 配置连接
	err = c.configureConnection(c.conn)
	if err != nil {
		return err
	}

	// 读取消息
	c.router = router
	go c.readHandler(ctx, c.session)

	// 连接成功处理
	go c.onConnected(NewContext(c.session, nil))

	err = <-c.stopChan
	c.beforeShutdown()
	c.onDisconnected(c.session, err)
	return err
}

func (c *Client) readHandler(ctx context.Context, session *Session) {
	c.wg.Add(1)
	defer c.wg.Done()

	var exited bool
	go func() {
		for {
			msg, err := ReadMsg(session)
			if err != nil {
				if exited {
					return
				}
				c.stopChan <- err
				return
			}
			c.onMessage(session, msg)
		}
	}()
	<-ctx.Done()
	exited = true
}
