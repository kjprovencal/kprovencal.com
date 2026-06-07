package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

const (
	defaultLogTailLines     = 200
	maxLogTailLines         = 500
	defaultLogTailReadBytes = 512 << 10 // 512 KiB from end of file
)

// tailLogFile returns up to maxLines complete lines from the end of path.
func tailLogFile(path string, maxLines int, maxReadBytes int64) ([]string, error) {
	if maxLines <= 0 {
		maxLines = defaultLogTailLines
	}
	if maxLines > maxLogTailLines {
		maxLines = maxLogTailLines
	}
	if maxReadBytes <= 0 {
		maxReadBytes = defaultLogTailReadBytes
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return []string{}, nil
	}

	readSize := info.Size()
	if readSize > maxReadBytes {
		readSize = maxReadBytes
	}
	if _, err := f.Seek(-readSize, io.SeekEnd); err != nil {
		return nil, err
	}

	buf := make([]byte, readSize)
	if _, err := io.ReadFull(f, buf); err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	// Drop a partial first line when we did not read the whole file.
	if readSize < info.Size() {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		} else {
			buf = nil
		}
	}

	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(buf))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
