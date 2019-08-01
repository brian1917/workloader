package utils

import (
	"bufio"
	"io"
)

// ClearBOM returns an io.Reader that will skip over initial UTF-8 byte order marks.
func ClearBOM(r io.Reader) io.Reader {
	buf := bufio.NewReader(r)
	b, err := buf.Peek(3)
	if err != nil {
		// not enough bytes
		return buf
	}
	if b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf {
		buf.Discard(3)
	}
	return buf
}
