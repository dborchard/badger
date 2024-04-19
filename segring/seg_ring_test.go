package segring

import (
	"context"
	"github.com/dgraph-io/badger/y"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// Test1 STD test. (Using Delay)
// Writes: [ts-3, ts-2, ts-1, ts]
// Read: scan(l,r,ts), scan(l,r+1,ts)
func Test1(t *testing.T) {

	tbl := New(15*time.Second, 60*time.Second, context.Background())

	tbl.Put(y.KeyWithTs([]byte("1"), uint64(time.Now().UnixNano())), y.ValueStruct{Value: []byte("a")}) // 10
	time.Sleep(1 * time.Second)
	tbl.Put(y.KeyWithTs([]byte("2"), uint64(time.Now().UnixNano())), y.ValueStruct{Value: []byte("b")}) // 11
	time.Sleep(1 * time.Second)
	tbl.Put(y.KeyWithTs([]byte("3"), uint64(time.Now().UnixNano())), y.ValueStruct{Value: []byte("c")}) // 12
	time.Sleep(1 * time.Second)
	tbl.Put(y.KeyWithTs([]byte("4"), uint64(time.Now().UnixNano())), y.ValueStruct{Value: []byte("d")}) // 13
	time.Sleep(1 * time.Second)

	now := time.Now()
	rows := tbl.Scan([]byte("1"), 1, uint64(now.UnixNano()))
	assert.Equal(t, 1, len(rows))
	assert.Equal(t, []byte("a"), rows[0].Val.Value)

	rows = tbl.Scan([]byte("1"), 2, uint64(now.UnixNano()))
	assert.Equal(t, 2, len(rows))
	assert.Equal(t, []byte("a"), rows[0].Val.Value)
	assert.Equal(t, []byte("b"), rows[1].Val.Value)

	rows = tbl.Scan([]byte("1"), 3, uint64(now.UnixNano()))
	assert.Equal(t, 3, len(rows))
	assert.Equal(t, []byte("a"), rows[0].Val.Value)
	assert.Equal(t, []byte("b"), rows[1].Val.Value)
	assert.Equal(t, []byte("c"), rows[2].Val.Value)

	rows = tbl.Scan([]byte("1"), 4, uint64(now.UnixNano()))
	assert.Equal(t, 4, len(rows))
	assert.Equal(t, []byte("a"), rows[0].Val.Value)
	assert.Equal(t, []byte("b"), rows[1].Val.Value)
	assert.Equal(t, []byte("c"), rows[2].Val.Value)
	assert.Equal(t, []byte("d"), rows[3].Val.Value)

	tbl.Close()
}
