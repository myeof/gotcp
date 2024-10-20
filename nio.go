package tcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/myeof/gotcp/pkg/logger"
)

func readLength(conn *net.TCPConn) (dataSize uint32, err error) {
	data := make([]byte, 4)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return 0, err
	}
	err = binary.Read(bytes.NewReader(data), binary.LittleEndian, &dataSize)
	if err != nil {
		return 0, err
	}
	return dataSize, nil
}

func ReadMsgId(conn *net.TCPConn) (msgId int32, err error) {
	data := make([]byte, 4)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return 0, err
	}
	err = binary.Read(bytes.NewReader(data), binary.LittleEndian, &msgId)
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
	// 读取UUID
	msg.requestID = make([]byte, 36)
	_, err = io.ReadFull(session.Conn(), msg.requestID)
	if err != nil {
		return nil, err
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
		// 读取head
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
		// 读取body
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

func WriteMsg(session *Session, msgID int32, headers, body []byte) error {
	if len(body) > MaxMsgSize {
		return errors.New("message too long")
	}
	uid := uuid.New()
	rid := []byte(uid.String())
	// 计算包头长度(排除body), 4字节总长度 + uuid + 4字节消息ID + 4字节head长度 + head + 4字节body长度
	headSize := 4 + 36 + 4 + 4 + len(headers) + 4
	// 预先分配缓冲区
	headerBuf := make([]byte, headSize)
	offset := 0
	// 写入总长度
	binary.LittleEndian.PutUint32(headerBuf[offset:offset+4], uint32(headSize+len(body)))
	offset += 4
	// 写入RequestID
	copy(headerBuf[offset:offset+36], rid)
	offset += 36
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
	// 限速
	var t *time.Timer
	if sendRateLimiter != nil {
		now := time.Now()
		r := sendRateLimiter.ReserveN(now, len(body))
		if r.OK() {
			delay := r.DelayFrom(now)
			if delay > 0 {
				t = time.NewTimer(delay)
				defer t.Stop()
			}
		} else {
			logger.Warn("sendRateLimiter.ReserveN error")
		}
	}
	// 发送数据, 加锁，防止并发发送
	session.Lock()
	defer session.Unlock()
	//st := time.Now()
	// 发送header
	_, err := session.Conn().Write(headerBuf)
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
	//logger.Infow("send message",
	//	"to", session.Remote(),
	//	"msgID", msgID,
	//	"cost", time.Since(st),
	//	"requestID", uid.String())
	// 等待发送完成
	if t != nil {
		<-t.C
	}
	return nil
}

func WriteMsgWithContext(ctx context.Context, session *Session, msgID int32, headers, body []byte) error {
	if len(body) > MaxMsgSize {
		return errors.New("message too long")
	}
	uid := uuid.New()
	rid := []byte(uid.String())
	// 计算包头长度(排除body), 4字节总长度 + uuid + 4字节消息ID + 4字节head长度 + head + 4字节body长度
	headSize := 4 + 36 + 4 + 4 + len(headers) + 4
	// 预先分配缓冲区
	headerBuf := make([]byte, headSize)
	offset := 0
	// 写入总长度
	binary.LittleEndian.PutUint32(headerBuf[offset:offset+4], uint32(headSize+len(body)))
	offset += 4
	// 写入RequestID
	copy(headerBuf[offset:offset+36], rid)
	offset += 36
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
	// 限速
	var t *time.Timer
	if sendRateLimiter != nil {
		now := time.Now()
		r := sendRateLimiter.ReserveN(now, len(body))
		if r.OK() {
			delay := r.DelayFrom(now)
			if delay > 0 {
				t = time.NewTimer(delay)
				defer t.Stop()
			}
		} else {
			logger.Warn("sendRateLimiter.ReserveN error")
		}
	}
	// 发送数据, 加锁，防止并发发送
	session.Lock()
	defer session.Unlock()
	deadline, ok := ctx.Deadline()
	if ok {
		err := session.Conn().SetWriteDeadline(deadline)
		if err != nil {
			return err
		}
	}
	// 发送header
	_, err := session.Conn().Write(headerBuf)
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
	// 等待发送完成
	if t != nil {
		<-t.C
	}
	return nil
}
