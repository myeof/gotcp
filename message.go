package tcp

import (
	"fmt"
)

const MaxMsgSize = 1024 * 1024 * 2 // 1MB

type Message struct {
	size uint32

	id int32

	headLength uint32
	header     []byte

	bodyLength uint32
	body       []byte
}

func NewMessage() *Message {
	return &Message{}
}

func (msg *Message) ID() int32 {
	return msg.id
}

func (msg *Message) Header() []byte {
	return msg.header
}

func (msg *Message) Body() []byte {
	return msg.body
}

func (msg *Message) String() string {
	return fmt.Sprintf("ID=%d HeadLan=%d DataLen=%d",
		msg.id, msg.headLength, msg.bodyLength)
}
