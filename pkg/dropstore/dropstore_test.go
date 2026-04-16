package dropstore

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"a.txt":            "a.txt",
		"/etc/passwd":      "passwd",
		`..\..\boot.ini`:   "boot.ini",
		" leading.txt ":    "leading.txt",
		"x\x00y.bin":       "xy.bin",
		"":                 "",
		"..":               "",
		"...":              "",
		"sub/dir/name.png": "name.png",
	}
	for in, want := range cases {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSaveConcurrent(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	paths := make([]string, N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			p, err := s.Save("same.txt", bytes.NewReader([]byte{'x'}))
			if err != nil {
				t.Errorf("save: %v", err)
				return
			}
			paths[i] = p
		}(i)
	}
	wg.Wait()

	// all paths must be unique and live under dir
	seen := map[string]bool{}
	for _, p := range paths {
		if !strings.HasPrefix(p, dir) {
			t.Errorf("path not under dir: %s", p)
		}
		if seen[p] {
			t.Errorf("duplicate path: %s", p)
		}
		seen[p] = true

		// sanity: file exists and contains the byte we wrote
		data, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(data, []byte{'x'}) {
			t.Errorf("bad file %s: %v %q", p, err, data)
		}
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != N {
		t.Errorf("dir has %d entries, want %d", len(entries), N)
	}

	// all filenames should end with "_same.txt"
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_same.txt") {
			t.Errorf("unexpected name: %s", e.Name())
		}
	}

	_ = filepath.Join // keep import used even if unused after edits
}
