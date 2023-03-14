package utils

import (
	"io"
	"os"
	"strings"
)

type WrappedLogger struct {
	out io.Writer
}

func NewWrappedLogger(writer io.Writer) *WrappedLogger {
	return &WrappedLogger{out: writer}
}

func (w *WrappedLogger) Write(p []byte) (n int, err error) {
	if strings.Contains(string(p), "connection reset by peer") && strings.Contains(string(p), "TLS handshake error") {
		return len(p), nil
	}
	if w.out == nil {
		w.out = os.Stderr
	}
	return w.out.Write(p)
}
