package segring

import (
	"container/list"
	"github.com/dgraph-io/badger/y"
	"github.com/tidwall/btree"
	"io"
)

type SegIterator struct {
	iter btree.IterG[Pair[[]byte, *list.Element]]
	err  error
}

var _ y.Iterator = new(SegIterator)

func (s *SegIterator) Next() {
	moved := s.iter.Next()
	if !moved {
		s.err = io.EOF
	}
}

func (s *SegIterator) Rewind() {
	s.iter.First()
}

func (s *SegIterator) Seek(key []byte) {
	forwarded := s.iter.Seek(Pair[[]byte, *list.Element]{Key: key})
	if !forwarded {
		s.err = io.EOF
	}
}

func (s *SegIterator) Key() []byte {
	return s.iter.Item().Key
}

func (s *SegIterator) Value() y.ValueStruct {
	return s.iter.Item().Val.Value.(y.ValueStruct)
}

func (s *SegIterator) Valid() bool {
	return s.err == nil
}

func (s *SegIterator) Close() error {
	s.iter.Release()
	return nil
}

func (s *SegIterator) SeekToFirst() {
	moved := s.iter.First()
	if !moved {
		s.err = io.EOF
	}
}
