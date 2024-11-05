package nixcmd

import (
	"bytes"
	"io"
	"sync"
)

// ErrorBufferCloser renders the error buffer contents if
// err is not nil, and then closes the error buffer.
//
// The error returned is the error that was passed in.
type ErrorBufferCloser func(err error) error

// ErrorBuffer returns a buffer that can be used to capture
// errors and then render them to stderr.
func ErrorBuffer(stdout, stderr io.Writer) (w io.Writer, closer ErrorBufferCloser) {
	buf := bytes.NewBuffer(nil)
	return newCombinerWriter(buf), func(err error) error {
		if err != nil {
			stderr.Write(buf.Bytes())
		}
		buf.Reset()
		return err
	}
}

func newCombinerWriter(w io.Writer) *combinerWriter {
	return &combinerWriter{
		m: &sync.Mutex{},
		w: w,
	}
}

type combinerWriter struct {
	m *sync.Mutex
	w io.Writer
}

func (cw *combinerWriter) Write(p []byte) (n int, err error) {
	cw.m.Lock()
	defer cw.m.Unlock()
	return cw.w.Write(p)
}
