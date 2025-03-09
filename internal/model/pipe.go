package model

import "os"

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
