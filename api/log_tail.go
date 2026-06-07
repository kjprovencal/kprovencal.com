package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"time"
)

const (
	defaultLogTailLines     = 200
	maxLogTailLines         = 500
	defaultLogTailReadBytes = 512 << 10 // 512 KiB from end of file
	maxLogFileScanBytes     = 16 << 20 // 16 MiB max read when filtering by time
)

type logQueryOpts struct {
	limit        int
	maxTailBytes int64
	since        *time.Time
	until        *time.Time
}

type logQueryResult struct {
	lines     []string
	truncated bool
	matched   int
}

// parseLogLineTime extracts the slog TextHandler time= field from a line.
func parseLogLineTime(line string) (time.Time, bool) {
	const prefix = "time="
	if !strings.HasPrefix(line, prefix) {
		return time.Time{}, false
	}
	rest := line[len(prefix):]
	end := strings.IndexByte(rest, ' ')
	if end < 0 {
		end = len(rest)
	}
	raw := strings.TrimSpace(rest[:end])
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func filterLogLinesByRange(lines []string, since, until *time.Time) []string {
	untilBound := time.Now().UTC()
	if until != nil {
		untilBound = until.UTC()
	}
	var sinceBound time.Time
	if since != nil {
		sinceBound = since.UTC()
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		t, ok := parseLogLineTime(line)
		if !ok {
			continue
		}
		t = t.UTC()
		if !sinceBound.IsZero() && t.Before(sinceBound) {
			continue
		}
		if t.After(untilBound) {
			continue
		}
		out = append(out, line)
	}
	return out
}

// readLogFileLines reads up to maxBytes from path. fromEnd=true reads the file tail.
func readLogFileLines(path string, maxBytes int64, fromEnd bool) ([]string, bool, error) {
	if maxBytes <= 0 {
		maxBytes = defaultLogTailReadBytes
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if info.Size() == 0 {
		return []string{}, false, nil
	}

	truncated := info.Size() > maxBytes
	readSize := info.Size()
	if readSize > maxBytes {
		readSize = maxBytes
	}

	if fromEnd {
		if _, err := f.Seek(-readSize, io.SeekEnd); err != nil {
			return nil, false, err
		}
	} else if truncated {
		// Range scan on a large file: use the most recent maxBytes only.
		if _, err := f.Seek(-readSize, io.SeekEnd); err != nil {
			return nil, false, err
		}
	} else {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, false, err
		}
	}

	buf := make([]byte, readSize)
	if _, err := io.ReadFull(f, buf); err != nil && err != io.ErrUnexpectedEOF {
		return nil, false, err
	}

	if fromEnd || truncated {
		if readSize < info.Size() {
			if i := bytes.IndexByte(buf, '\n'); i >= 0 {
				buf = buf[i+1:]
			} else {
				buf = nil
			}
		}
	}

	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(buf))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, false, err
	}
	return lines, truncated, nil
}

func queryLogFile(path string, opts logQueryOpts) (logQueryResult, error) {
	if opts.limit <= 0 {
		opts.limit = defaultLogTailLines
	}
	if opts.limit > maxLogTailLines {
		opts.limit = maxLogTailLines
	}
	if opts.maxTailBytes <= 0 {
		opts.maxTailBytes = defaultLogTailReadBytes
	}

	hasRange := opts.since != nil || opts.until != nil

	var (
		raw       []string
		truncated bool
		err       error
	)
	if hasRange {
		raw, truncated, err = readLogFileLines(path, maxLogFileScanBytes, false)
	} else {
		raw, truncated, err = readLogFileLines(path, opts.maxTailBytes, true)
	}
	if err != nil {
		return logQueryResult{}, err
	}

	lines := raw
	if hasRange {
		lines = filterLogLinesByRange(raw, opts.since, opts.until)
	}

	matched := len(lines)
	if len(lines) > opts.limit {
		lines = lines[len(lines)-opts.limit:]
	}

	return logQueryResult{
		lines:     lines,
		truncated: truncated,
		matched:   matched,
	}, nil
}

// tailLogFile returns up to maxLines complete lines from the end of path.
func tailLogFile(path string, maxLines int, maxReadBytes int64) ([]string, error) {
	res, err := queryLogFile(path, logQueryOpts{
		limit:        maxLines,
		maxTailBytes: maxReadBytes,
	})
	if err != nil {
		return nil, err
	}
	return res.lines, nil
}
