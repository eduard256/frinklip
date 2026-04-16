package upload

import (
	"os"
	"time"
)

func fileSize(path string) int64 {
	if info, err := os.Stat(path); err == nil {
		return info.Size()
	}
	return 0
}

func nowUnix() int64 {
	return time.Now().Unix()
}
