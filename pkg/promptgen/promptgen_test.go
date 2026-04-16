package promptgen

import "testing"

func TestIsImage(t *testing.T) {
	cases := map[string]bool{
		"/tmp/a.jpg":         true,
		"/tmp/a.JPG":         true,
		"/tmp/a.jpeg":        true,
		"/tmp/a.png":         true,
		"/tmp/a.gif":         true,
		"/tmp/a.webp":        true,
		"/tmp/a.bmp":         true,
		"/tmp/a.svg":         true,
		"/tmp/a.heic":        false,
		"/tmp/a.pdf":         false,
		"/tmp/a.txt":         false,
		"/tmp/noext":         false,
		"/tmp/dir.png/a.txt": false,
	}
	for in, want := range cases {
		if got := IsImage(in); got != want {
			t.Errorf("IsImage(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBuild(t *testing.T) {
	got := Build([]string{"/tmp/a.pdf", "/tmp/b.jpg"})
	want := "Посмотри файл /tmp/a.pdf\nПрочитай изображение /tmp/b.jpg"
	if got != want {
		t.Errorf("Build mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}
