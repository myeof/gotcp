package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	tcp "github.com/myeof/gotcp"
)

const (
	PingMsgId = iota + 1
	TextMsgId
	JSONMsgId
	ReqFileMsgId
	BinaryMsgId
)

var host = "127.0.0.1:8080"

func init() {
	log.SetPrefix(fmt.Sprintf("[%d] ", os.Getpid()))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	var s bool
	flag.BoolVar(&s, "s", s, "server")
	flag.Parse()
	if s {
		ServerExample()
	} else {
		ClientExample()
	}
}

// examples

func ServerExample() {
	// 消息路由
	r := tcp.NewRouter()
	r.Use(LogHandler)
	register(r)

	// 服务端
	s := tcp.NewServer()
	s.SetOnConnected(func(c *tcp.Context) {
		log.Println("connected", c.Remote())
	})
	s.SetOnDisconnect(func(s *tcp.Session, err error) {
		log.Println("disconnected", s.Remote())
	})

	// 监听端口
	listener, err := s.Listen(host)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("listen", listener.Addr().String())

	// 开始服务
	err = s.Serve(r)
	if err != nil {
		log.Fatalln(err)
	}
}

func ClientExample() {
	r := tcp.NewRouter()
	register(r)

	// 客户端
	c := tcp.NewClient()
	c.SetOnDisconnect(func(s *tcp.Session, err error) {
		log.Println("disconnected", s.Remote())
	})
	c.SetOnConnected(Start)

	// 连接服务端
	err := c.Connect(host, r)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("server closed")
		} else if errors.Is(err, net.ErrClosed) {
			log.Printf("client closed")
		} else {
			log.Fatalln(err)
		}
	}
}

// routers

func register(r *tcp.Router) {
	r.Register(PingMsgId, Pong, Pong)
	r.Register(TextMsgId, Text)
	r.Register(JSONMsgId, JSON)
	r.Register(ReqFileMsgId, HandleRequestFile)
	r.Register(BinaryMsgId, HandleSaveFile)
}

// handlers

func LogHandler(c *tcp.Context) {
	startTime := time.Now()
	c.Next()
	log.Printf("| %15s | %3d | %8d | %10s\n",
		c.Remote(),
		c.MsgID(),
		c.MsgSize(),
		time.Since(startTime),
	)
}

func Start(c *tcp.Context) {
	log.Println("start")
	var err error
	var input string
	for {
		fmt.Printf("请输入内容: ")
		_, _ = fmt.Scanln(&input)
		input = strings.Trim(input, "\r\n")
		switch input {
		case "":
			continue
		case "help":
			fmt.Println("exit: 退出")
			fmt.Println("ping: 发送Ping")
			fmt.Println("json: 发送JSON")
			fmt.Println("file: 请求文件")
			fmt.Println("*: 发送文本")
		case "exit":
			_ = c.Close()
			return
		case "ping":
			err = c.SendText(PingMsgId, "Ping")
		case "json":
			err = c.SendJSON(JSONMsgId, map[string]interface{}{
				"code": 0,
			})
		case "file":
			err = c.SendJSON(ReqFileMsgId, map[string]interface{}{
				"file":   "example/example.go",
				"length": 100,
				"offset": 0,
			})
		default:
			err = c.SendText(TextMsgId, input)
		}
		if err != nil {
			log.Println(err)
		}
		input = ""
	}
}

func Pong(c *tcp.Context) {
	_ = c.SendText(TextMsgId, "Pong")
	c.Abort()
}

func Text(c *tcp.Context) {
	log.Printf("%s: %s", c.Remote(), c.Text())
	_ = c.SendJSON(0, map[string]interface{}{
		"code": 0,
	})
}

func JSON(c *tcp.Context) {
	var body map[string]interface{}
	err := c.BindJSON(&body)
	if err != nil {
		_ = c.SendText(TextMsgId, err.Error())
		return
	}
	log.Printf("%s: %#v", c.Remote(), body)
}

func HandleRequestFile(c *tcp.Context) {
	var body map[string]interface{}
	err := c.BindJSON(&body)
	if err != nil {
		_ = c.SendText(TextMsgId, err.Error())
		return
	}
	filename, ok := body["file"].(string)
	if !ok {
		_ = c.SendText(TextMsgId, "error: filename")
		return
	}
	length, ok := body["length"].(float64)
	if !ok || length == 0 {
		_ = c.SendText(TextMsgId, "error: length")
		return
	}
	offset, ok := body["offset"].(float64)
	if !ok {
		offset = 0
	}
	if int(length) > 1024*1024*5 {
		_ = c.SendText(TextMsgId, "error: length too long")
		return
	}

	var data = make([]byte, int(length))
	f, err := os.Open(filename)
	if err != nil {
		_ = c.SendText(TextMsgId, err.Error())
		return
	}

	n, err := f.ReadAt(data, int64(offset))
	if err != nil {
		_ = c.SendText(TextMsgId, err.Error())
		return
	}

	_ = c.SendStream(BinaryMsgId,
		map[string]interface{}{
			"offset": offset,
			"length": n,
		},
		data[:n],
	)
}

func HandleSaveFile(c *tcp.Context) {
	var headers map[string]interface{}
	err := c.HeaderBindJSON(&headers)
	if err != nil {
		_ = c.SendText(TextMsgId, err.Error())
		return
	}

	f, err := os.Create("./save.txt")
	if err != nil {
		_ = c.SendText(TextMsgId, err.Error())
		return
	}

	offset, ok := headers["offset"].(float64)
	if ok {
		_, _ = f.Seek(int64(offset), 0)
	}

	length, ok := headers["length"].(float64)
	if ok {
		log.Printf("length: %f", length)
	}

	_, err = c.CopyTo(f)

	if err != nil {
		_ = c.SendText(TextMsgId, err.Error())
	}
}
