package data

import (
	"bytes"
	"io"
	"os"
	"time"
)

// TailFile opens a file, seeks to the given offset, and emits complete lines
// (terminated by \n) on the returned channel. Incomplete trailing data is
// buffered until the next \n arrives. The goroutine stops when stopCh is closed.
func TailFile(path string, offset int64, stopCh <-chan struct{}) <-chan []byte {
	lines := make(chan []byte, 16)
	go func() {
		defer close(lines)

		var f *os.File
		var err error
		for {
			f, err = os.Open(path)
			if err == nil {
				break
			}
			select {
			case <-stopCh:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
		defer f.Close()

		if offset > 0 {
			f.Seek(offset, io.SeekStart)
		}

		var buf []byte
		tmp := make([]byte, 32*1024)

		for {
			n, err := f.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				for {
					idx := bytes.IndexByte(buf, '\n')
					if idx < 0 {
						break
					}
					line := make([]byte, idx)
					copy(line, buf[:idx])
					buf = buf[idx+1:]

					select {
					case lines <- line:
					case <-stopCh:
						return
					}
				}
			}
			if err == io.EOF || n == 0 {
				select {
				case <-stopCh:
					return
				case <-time.After(200 * time.Millisecond):
				}
				continue
			}
			if err != nil {
				return
			}
		}
	}()
	return lines
}

// FileSize returns the current size of a file.
func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
