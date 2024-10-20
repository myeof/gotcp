package tcp

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/myeof/gotcp/pkg/logger"
	"github.com/myeof/gotcp/worker"
)

const (
	StateInit = iota
	StateRunning
	StateShuttingDown
	StateTerminate
)

type connBase struct {
	// addr 服务端监听地址 或 客户端连接地址
	addr *net.TCPAddr

	// wg 用于等待所有goroutine退出
	wg sync.WaitGroup

	state      uint8
	stopChan   chan error
	showLog    bool // 是否打印日志
	workerNum  int
	worker     *worker.Worker
	hbInterval time.Duration

	router *Router

	connectedHandler      func(c *Context)
	disconnectHandler     func(session *Session, err error)
	beforeShutdownHandler func()
}

func (b *connBase) SetWorker(w int) {
	b.workerNum = w
	b.worker = worker.NewWorker(b.workerNum, b.workerNum*2)
}

func (b *connBase) SetOnConnected(f func(c *Context)) {
	b.connectedHandler = f
}

func (b *connBase) SetOnDisconnect(f func(s *Session, err error)) {
	b.disconnectHandler = f
}

func (b *connBase) SetBeforeShutdown(f func()) {
	b.beforeShutdownHandler = f
}

func (b *connBase) onConnected(ctx *Context) {
	if b.connectedHandler != nil {
		b.connectedHandler(ctx)
	}
}

func (b *connBase) onDisconnected(session *Session, err error) {
	if b.disconnectHandler != nil {
		b.disconnectHandler(session, err)
	}
}

func (b *connBase) beforeShutdown() {
	if b.beforeShutdownHandler != nil {
		b.beforeShutdownHandler()
	}
}

func (b *connBase) configureConnection(conn *net.TCPConn) error {
	var err error
	err = conn.SetKeepAlive(true)
	if err != nil {
		return err
	}
	err = conn.SetKeepAlivePeriod(b.hbInterval)
	return err
}

func (b *connBase) onMessage(session *Session, msg *Message) {
	b.worker.StartJob(func() {
		c := NewContext(session, msg)

		handlers := b.router.GetHandlers(c.MsgID())
		if len(handlers) == 0 {
			logger.Warnf("No handler for message id: %d", c.MsgID())
			return
		}
		c.handlers = append(b.router.GetMiddlewares(), handlers...)
		// 执行消息处理函数
		for c.index < int8(len(handlers)) {
			c.handlers[c.index](c)
			c.index++
		}
	})
}

func (b *connBase) terminal() {
	if b.state != StateRunning {
		return
	}
	b.state = StateShuttingDown
	b.stopChan <- errors.New("terminal")
}

func (b *connBase) handleSignals(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	defer close(sigChan)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGTSTP)
	defer signal.Stop(sigChan)
	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				logger.Info("Received SIGHUP.")
			case syscall.SIGINT:
				logger.Info("Received SIGINT.")
				b.terminal()
				return
			case syscall.SIGTERM:
				logger.Info("Received SIGTERM.")
				b.terminal()
				return
			case syscall.SIGUSR1:
				logger.Info("Received SIGUSR1.")
			case syscall.SIGUSR2:
				logger.Info("Received SIGUSR2.")
			case syscall.SIGTSTP:
				logger.Info("Received SIGTSTP.")
			default:
				logger.Infof("Received %v: nothing i care about...", sig)
			}
		}
	}
}
