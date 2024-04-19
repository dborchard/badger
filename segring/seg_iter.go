package segring

import "github.com/dgraph-io/badger/y"

type SegIterator struct {
}

var _ y.Iterator = new(SegIterator)

func (s *SegIterator) Next() {
	//TODO implement me
	panic("implement me")
}

func (s *SegIterator) Rewind() {
	//TODO implement me
	panic("implement me")
}

func (s *SegIterator) Seek(key []byte) {
	//TODO implement me
	panic("implement me")
}

func (s *SegIterator) Key() []byte {
	//TODO implement me
	panic("implement me")
}

func (s *SegIterator) Value() y.ValueStruct {
	//TODO implement me
	panic("implement me")
}

func (s *SegIterator) Valid() bool {
	//TODO implement me
	panic("implement me")
}

func (s *SegIterator) Close() error {
	//TODO implement me
	panic("implement me")
}

func (s *SegIterator) SeekToFirst() {

}
