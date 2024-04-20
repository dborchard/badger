package timez

import (
	"time"
)

var (
	ts uint64
	//mu sync.Mutex
)

func init() {
	ts = 1
}

func Now() uint64 {
	//mu.Lock()
	//defer mu.Unlock()
	ts++
	return ts + uint64(time.Now().UnixNano())
}
