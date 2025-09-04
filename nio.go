package tcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/myeof/gotcp/pkg/logger"
)

// write

// writeMessageHeader 构建消息头部到指定的缓冲区
func writeMessageHeader(headerBuf []byte, msgID int32, headers, body []byte) error {
	// 计算包头长度: 4字节总长度 + 4字节消息ID + 4字节head长度 + head + 4字节body长度
	headSize := 4 + 4 + 4 + len(headers) + 4
	totalSize := headSize + len(body)

	// 确保缓冲区有足够空间
	if len(headerBuf) < headSize {
		return errors.New("buffer too small")
	}

	offset := 0
	// 写入总长度
	binary.LittleEndian.PutUint32(headerBuf[offset:offset+4], uint32(totalSize))
	offset += 4
	// 写入消息ID
	binary.LittleEndian.PutUint32(headerBuf[offset:offset+4], uint32(msgID))
	offset += 4
	// 写入头长度
	binary.LittleEndian.PutUint32(headerBuf[offset:offset+4], uint32(len(headers)))
	offset += 4
	// 写入头部
	if len(headers) > 0 {
		copy(headerBuf[offset:offset+len(headers)], headers)
		offset += len(headers)
	}
	// 写入体长度
	binary.LittleEndian.PutUint32(headerBuf[offset:offset+4], uint32(len(body)))

	return nil
}

// applyRateLimit 应用发送限速
func applyRateLimit(bodySize int) *time.Timer {
	if sendRateLimiter == nil {
		return nil
	}

	now := time.Now()
	r := sendRateLimiter.ReserveN(now, bodySize)
	if !r.OK() {
		logger.Warn("sendRateLimiter.ReserveN error")
		return nil
	}

	delay := r.DelayFrom(now)
	if delay > 0 {
		t := time.NewTimer(delay)
		return t
	}
	return nil
}

func WriteMsg(session *Session, msgID int32, headers, body []byte) error {
	if len(body) > MaxMsgSize {
		return errors.New("message too long")
	}

	// 计算需要的头部缓冲区大小
	headSize := 4 + 4 + 4 + len(headers) + 4

	// 从对象池获取缓冲区
	headerBuf := bufferPool.Get(headSize)
	defer bufferPool.Put(headerBuf)

	// 构建消息头部到池中的缓冲区
	err := writeMessageHeader(headerBuf, msgID, headers, body)
	if err != nil {
		return err
	}

	// 应用限速
	t := applyRateLimit(len(body))
	if t != nil {
		defer t.Stop()
	}

	// 发送数据，加锁防止并发发送
	session.Lock()
	defer session.Unlock()

	// 发送header（只发送实际使用的部分）
	_, err = session.Conn().Write(headerBuf[:headSize])
	if err != nil {
		return err
	}

	// 发送body
	if len(body) > 0 {
		_, err = session.Conn().Write(body)
		if err != nil {
			return err
		}
	}

	// 等待限速完成
	if t != nil {
		<-t.C
	}

	return nil
}

func WriteMsgWithContext(ctx context.Context, session *Session, msgID int32, headers, body []byte) error {
	if len(body) > MaxMsgSize {
		return errors.New("message too long")
	}

	// 计算需要的头部缓冲区大小
	headSize := 4 + 4 + 4 + len(headers) + 4

	// 从对象池获取缓冲区
	headerBuf := bufferPool.Get(headSize)
	defer bufferPool.Put(headerBuf)

	// 构建消息头部到池中的缓冲区
	err := writeMessageHeader(headerBuf, msgID, headers, body)
	if err != nil {
		return err
	}

	// 应用限速
	t := applyRateLimit(len(body))
	if t != nil {
		defer t.Stop()
	}

	// 发送数据，加锁防止并发发送
	session.Lock()
	defer session.Unlock()

	// 设置写入超时
	deadline, ok := ctx.Deadline()
	if ok {
		err := session.Conn().SetWriteDeadline(deadline)
		if err != nil {
			return err
		}
	}

	// 发送header（只发送实际使用的部分）
	_, err = session.Conn().Write(headerBuf[:headSize])
	if err != nil {
		return err
	}

	// 发送body
	if len(body) > 0 {
		_, err = session.Conn().Write(body)
		if err != nil {
			return err
		}
	}

	// 等待限速完成
	if t != nil {
		<-t.C
	}

	return nil
}

// read

func readLength(conn *net.TCPConn) (dataSize uint32, err error) {
	// 使用对象池：数据读取后立即使用，生命周期很短
	buf := bufferPool.Get(4)
	defer bufferPool.Put(buf)

	_, err = io.ReadFull(conn, buf)
	if err != nil {
		return 0, err
	}
	err = binary.Read(bytes.NewReader(buf), binary.LittleEndian, &dataSize)
	if err != nil {
		return 0, err
	}
	return dataSize, nil
}

func ReadMsgId(conn *net.TCPConn) (msgId int32, err error) {
	// 使用对象池：数据读取后立即使用，生命周期很短
	buf := bufferPool.Get(4)
	defer bufferPool.Put(buf)

	_, err = io.ReadFull(conn, buf)
	if err != nil {
		return 0, err
	}
	err = binary.Read(bytes.NewReader(buf), binary.LittleEndian, &msgId)
	if err != nil {
		return 0, err
	}
	return msgId, nil
}

func ReadMsg(session *Session) (*Message, error) {
	var err error
	var msg = NewMessage()
	// 读取总长度
	msgSize, err := readLength(session.Conn())
	if err != nil {
		return nil, err
	}
	if msgSize > MaxMsgSize {
		return nil, errors.New("message too long")
	}
	msg.size = msgSize
	// 限速
	var t *time.Timer
	if receiveRateLimiter != nil {
		now := time.Now()
		r := receiveRateLimiter.ReserveN(now, int(msgSize))
		if r.OK() {
			delay := r.DelayFrom(now)
			if delay > 0 {
				t = time.NewTimer(delay)
				defer t.Stop()
			}
		} else {
			logger.Warn("receiveRateLimiter.ReserveN error")
		}
	}
	// 读取消息id
	msg.id, err = ReadMsgId(session.Conn())
	if err != nil {
		return nil, err
	}
	// 读取head长度
	headLength, err := readLength(session.Conn())
	if err != nil {
		return nil, err
	}
	if headLength > 0 {
		// 读取head - 不能使用对象池，因为数据需要长期保存
		// 这些数据会传递给消息处理逻辑，生命周期较长
		msg.header = make([]byte, headLength)
		_, err = io.ReadFull(session.Conn(), msg.header)
		if err != nil {
			return nil, err
		}
	}
	// 读取body长度
	bodyLength, err := readLength(session.Conn())
	if err != nil {
		return nil, err
	}
	if bodyLength > 0 {
		// 读取body - 不能使用对象池，因为数据需要长期保存
		// 这些数据会传递给消息处理逻辑，生命周期较长
		msg.body = make([]byte, bodyLength)
		_, err = io.ReadFull(session.Conn(), msg.body)
		if err != nil {
			return nil, err
		}
	}
	if t != nil {
		<-t.C
	}
	return msg, nil
}
