package dropstore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Store writes uploaded files into Dir with a unique "timestamp_name" prefix
type Store struct {
	Dir string

	// counter disambiguates files that arrive within the same nanosecond
	counter atomic.Uint64
}

// New creates a Store and ensures Dir exists (0755)
func New(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("dropstore: empty dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// Save writes r into Dir using sanitized name with a unix-nano timestamp prefix
// and returns the absolute path.
func (s *Store) Save(name string, r io.Reader) (string, error) {
	name = SafeName(name)
	if name == "" {
		name = "file"
	}

	// unix seconds + nanosecond counter — short, monotonic, unique per process
	ts := time.Now().Unix()
	n := s.counter.Add(1)

	fname := strconv.FormatInt(ts, 10) + "_" + strconv.FormatUint(n, 10) + "_" + name
	path := filepath.Join(s.Dir, fname)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}

	if _, err = io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}

	if err = f.Close(); err != nil {
		return "", err
	}

	return path, nil
}

// SafeName strips path separators, control chars, and whitespace from a
// user-provided name. Whitespace is replaced with underscore so the final
// /tmp path never contains spaces — easier to paste into shells and prompts.
func SafeName(name string) string {
	// take basename only — multipart can legally include directories
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)

	// replace whitespace with "_", drop other control bytes
	b := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == ' ' || ch == '\t' {
			b = append(b, '_')
			continue
		}
		if ch < 0x20 || ch == 0x7f {
			continue
		}
		b = append(b, ch)
	}
	name = string(b)

	if name == "" || name == "." || name == ".." || strings.Trim(name, ".") == "" {
		return ""
	}
	return name
}
