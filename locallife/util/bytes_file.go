package util

import (
	"bytes"
	"mime/multipart"
)

type bytesFile struct {
	*bytes.Reader
}

func (f *bytesFile) Close() error {
	return nil
}

func NewBytesFile(data []byte) multipart.File {
	return &bytesFile{Reader: bytes.NewReader(data)}
}
