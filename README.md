# gotcp

一个简单的go tcp框架，用法参考了gin框架

## usage

完整示例[example.go](example%2Fexample.go)

### 1. 定义消息id

```go
package main

const (
	PingMsgId = iota + 1
	TextMsgId
	JSONMsgId
	ReqFileMsgId
	BinaryMsgId
)

```

### 2. 定义消息处理函数

```go
package main

import tcp "github.com/myeof/gotcp"

func Pong(c *tcp.Context) {
	_ = c.SendText(TextMsgId, "Pong")
}
```

### 3. 注册路由

```go
package main

import tcp "github.com/myeof/gotcp"

func register(r *tcp.Router) {
	r.Register(PingMsgId, Pong)
	r.Register(TextMsgId, Text)
	r.Register(JSONMsgId, JSON)
	...
}
```

### 4. 服务端

```go
package main

import (
	"log"

	tcp "github.com/myeof/gotcp"
)

func main() {
	r := tcp.NewRouter()
	r.Use(LogHandler)                              // Use，注册中间件
	register(r)                                    // 注册路由
	err := tcp.ListenAndServe("127.0.0.1:8080", r) // 启动监听和服务
	if err != nil {
		log.Fatalln(err)
	}
}
```

### 5. 客户端

```go
package main

import (
	"errors"
	"log"
	"net"

	tcp "github.com/myeof/gotcp"
)

func main() {
	r := tcp.NewRouter()
	register(r) // 注册路由
	c := tcp.NewClient()
	c.SetOnConnected(Start) // Start，连接服务端成功后执行的函数
	err := c.Connect("127.0.0.1:8080", r)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			log.Printf("exit")
		} else {
			log.Fatalln(err)
		}
	}
}
```

## 其他

- server.SetCPS 设置服务端每秒创建连接数
- InitRate 设置tcp收发包速率，方便控制带宽
