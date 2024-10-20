package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/myeof/gotcp"
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
	var c string
	flag.BoolVar(&s, "s", s, "server")
	flag.StringVar(&c, "c", "", "host")
	flag.Parse()
	if s {
		ServerExample()
	} else {
		ClientExample()
	}
}

// examples

func ServerExample() {
	r := tcp.NewRouter()
	r.Use(LogHandler)
	register(r)
	err := tcp.ListenAndServe(host, r)
	if err != nil {
		log.Fatalln(err)
	}
}

func ClientExample() {
	r := tcp.NewRouter()
	register(r)
	c := tcp.NewClient()
	c.SetOnConnected(Start)
	err := c.Connect(host, r)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			log.Printf("exit")
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
	log.Printf("| %15s | %s | %3d | %8d | %10s\n",
		c.Remote(),
		c.RequestID(),
		c.MsgID(),
		c.MsgSize(),
		time.Since(startTime),
	)
}

func Start(c *tcp.Context) {
	log.Println("start")
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
			_ = c.SendText(PingMsgId, "Ping")
		case "json":
			_ = c.SendJSON(JSONMsgId, map[string]interface{}{
				"code": 0,
			})
		case "file":
			_ = c.SendJSON(ReqFileMsgId, map[string]interface{}{
				"file":   "example/example.go",
				"length": 100,
				"offset": 0,
			})
		default:
			_ = c.SendText(TextMsgId, input)
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
	var err error

	var body map[string]interface{}
	_ = c.BindJSON(&body)
	filename, ok := body["file"].(string)
	if !ok {
		_ = c.SendText(TextMsgId, "error: filename")
		return
	}
	length, ok := body["length"].(float64)
	if !ok {
		length = 0
	}
	offset, ok := body["offset"].(float64)
	if !ok {
		offset = 0
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
		data[:n])
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
