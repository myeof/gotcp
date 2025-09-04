package tcp

import "sync"

// BufferPool 线程安全的缓冲区池
type BufferPool struct {
	pool sync.Pool
}

// 全局缓冲区池
var bufferPool = NewBufferPool()

// NewBufferPool 创建新的缓冲区池
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// 默认分配4KB缓冲区
				buf := make([]byte, 0, 4096)
				return &buf
			},
		},
	}
}

// Get 获取缓冲区，如果容量不足会自动扩容
func (p *BufferPool) Get(size int) []byte {
	bufPtr := p.pool.Get().(*[]byte)
	buf := *bufPtr
	if cap(buf) < size {
		// 如果容量不足，重新分配
		buf = make([]byte, 0, size)
	}
	return buf[:size] // 设置正确长度
}

// Put 归还缓冲区到池中
func (p *BufferPool) Put(buf []byte) {
	if cap(buf) > 0 {
		p.pool.Put(&buf)
	}
}
