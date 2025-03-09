package model

import (
	"bytes"
	"io"
	"os"
)

type Pipe struct {
	Reader *os.File
	Writer *os.File
}

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

func (p *Pipe) Write(s string) (int, error) {
	return p.Writer.Write([]byte(s))
}

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
