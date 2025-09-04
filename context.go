package tcp

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net"
)

const (
	abortIndex = math.MaxInt8 >> 1
)

type Context struct {
	session *Session
	msg     *Message

	handlers []func(ctx *Context)
	index    int8
}

func NewContext(session *Session, msg *Message) *Context {
	return &Context{
		session: session,
		msg:     msg,
	}
}

func (c *Context) Session() *Session {
	return c.session
}

func (c *Context) Conn() *net.TCPConn {
	return c.session.Conn()
}

func (c *Context) Remote() string {
	return c.session.Conn().RemoteAddr().String()
}

func (c *Context) Close() error {
	return c.session.Conn().Close()
}

// msg

func (c *Context) MsgID() int32 {
	return c.msg.ID()
}

func (c *Context) MsgSize() uint32 {
	return c.msg.size
}

// headers

func (c *Context) HeaderBindJSON(v interface{}) error {
	return json.Unmarshal(c.msg.header, v)
}

// body

func (c *Context) Text() string {
	return string(c.msg.body)
}

func (c *Context) BindJSON(v interface{}) error {
	return json.Unmarshal(c.msg.body, v)
}

func (c *Context) Reader() io.Reader {
	return bytes.NewReader(c.msg.body)
}

func (c *Context) CopyTo(writer io.Writer) (int64, error) {
	return io.Copy(writer, c.Reader())
}

// response

func (c *Context) SendJSON(msgID int32, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return WriteMsg(c.session, msgID, nil, b)
}

func (c *Context) SendText(msgID int32, data string) error {
	return WriteMsg(c.session, msgID, nil, []byte(data))
}

func (c *Context) SendStream(msgID int32, headers interface{}, data []byte) error {
	bHeader, err := json.Marshal(headers)
	if err != nil {
		return err
	}
	return WriteMsg(c.session, msgID, bHeader, data)
}

// handler

func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		c.handlers[c.index](c)
		c.index++
	}
}

func (c *Context) Abort() {
	c.index = abortIndex
}
