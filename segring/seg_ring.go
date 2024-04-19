package segring

import (
	"github.com/dgraph-io/badger/y"
)

type SegRing struct {
}

func (r *SegRing) Put(nk []byte, vs y.ValueStruct) {

}

func (r *SegRing) Empty() bool {
	return true
}

func (r *SegRing) MemSize() int64 {
	return 0
}

func (r *SegRing) NewIterator() *SegIterator {
	return nil
}

func New() *SegRing {
	return &SegRing{}
}
