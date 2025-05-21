package model

import (
	"bytes"
	"io"
	"os"
)

// Pipe 为读写管道的封装
type Pipe struct {
	Reader *os.File
	Writer *os.File
}

// Close 关闭读写管道
func (p *Pipe) Close() error {
	if p.Reader != nil {
		if err := p.Reader.Close(); err != nil {
			return err
		}
	}
	if p.Writer != nil {
		if err := p.Writer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Write 写入数据到管道
func (p *Pipe) Write(s string) (int, error) {
	return p.Writer.Write([]byte(s))
}

// Read 从管道读取数据
func (p *Pipe) Read() (string, error) {
	var buffer bytes.Buffer
	tmp := make([]byte, 4096)

	for {
		n, err := p.Reader.Read(tmp)
		if n > 0 {
			buffer.Write(tmp[:n]) // 自动处理内存增长
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return buffer.String(), err // 返回已读取的有效数据及错误
		}
	}
	return buffer.String(), nil
}

// CopyFrom 将任意Reader接口的数据持续写入管道Writer
// 参数：
//
//	src - 需要读取的输入源，可以是文件、网络连接、内存缓冲等
//
// 返回值：
//
//	int64 - 成功写入的字节数
//	error - 写入过程中遇到的任何错误
func (p *Pipe) CopyFrom(src io.Reader) (int64, error) {
	// 使用io.Copy实现自动缓冲和数据拷贝
	return io.Copy(p.Writer, src)
}

// NewPipe 创建一个读写管道
func NewPipe() (*Pipe, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &Pipe{
		Reader: reader,
		Writer: writer,
	}, nil
}
