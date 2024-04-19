package segring

import (
	"github.com/tidwall/btree"
	"sync/atomic"
)

type BTreeGCoW[T any] struct {
	state atomic.Pointer[btree.BTreeG[T]]
}

type IBTreeGCoW[T any] interface {
	Set(item T) (T, bool)
	Clear()
	Ascend(pivot T, iter func(item T) bool)
	Len() int
}

var _ IBTreeGCoW[any] = new(BTreeGCoW[any])

func NewBTreeGCoW[T any](less func(a, b T) bool) *BTreeGCoW[T] {
	r := BTreeGCoW[T]{}
	r.state.Store(btree.NewBTreeG[T](less))
	return &r
}

func (tr *BTreeGCoW[T]) Set(item T) (T, bool) {
	newState := tr.state.Load().Copy()
	res1, res2 := newState.Set(item)
	tr.state.Store(newState)
	return res1, res2
}

func (tr *BTreeGCoW[T]) Clear() {
	tr.state.Load().Clear()
}

func (tr *BTreeGCoW[T]) Ascend(pivot T, iter func(item T) bool) {
	tr.state.Load().Ascend(pivot, iter)
}

func (tr *BTreeGCoW[T]) Len() int {
	return tr.state.Load().Len()
}

func (tr *BTreeGCoW[T]) Iter() btree.IterG[T] {
	return tr.state.Load().Iter()
}
