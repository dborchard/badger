package segring

import (
	"container/list"
	"context"
	"github.com/alphadose/zenq/v2"
	"github.com/dgraph-io/badger/y"
	"runtime"
	"sync/atomic"
	"time"
)

type Segment struct {
	// Original Structure
	tree *BTreeGCoW[Pair[[]byte, *list.Element]]
	vlog *list.List
	ctx  context.Context

	// Ring Enhancement
	nextPtr *Segment

	// Async Logic
	asyncKeyPtrChan *zenq.ZenQ[*Pair[[]byte, *list.Element]]
	done            atomic.Bool
	pendingUpdates  atomic.Int64
}

func NewSegment(ctx context.Context) *Segment {
	segment := Segment{
		tree: NewBTreeGCoW(func(a, b Pair[[]byte, *list.Element]) bool {
			return CompareKeys(a.Key, b.Key) < 0
		}),
		vlog:            list.New(),
		ctx:             ctx,
		asyncKeyPtrChan: zenq.New[*Pair[[]byte, *list.Element]](1 << 20),
		done:            atomic.Bool{},
	}

	return &segment
}

func (s *Segment) StartListener() {
	// Ctx Listener
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				s.done.Store(true)
				return
			}
		}
	}()

	// Writer Thread
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		var entry *Pair[[]byte, *list.Element]
		var isQueueOpen bool

		defer s.asyncKeyPtrChan.Close()
		for {

			if entry, isQueueOpen = s.asyncKeyPtrChan.Read(); isQueueOpen {
				s.tree.Set(*entry)
				s.pendingUpdates.Add(-1)
			}

			if s.done.Load() {
				return
			}
		}
	}()
}

func (s *Segment) AddValue(val y.ValueStruct) (lePtr *list.Element) {
	return s.vlog.PushFront(val)
}

func (s *Segment) AddIndex(entry *Pair[[]byte, *list.Element]) {
	// For explicit serialization
	delay := time.Duration(1)
	for s.pendingUpdates.Load() > 0 {
		// Waiting time was generally between 10-250ms
		time.Sleep(delay * time.Millisecond)
		delay = delay * 2
	}

	s.tree.Set(*entry)
}

func (s *Segment) AddIndexAsync(entry *Pair[[]byte, *list.Element]) {
	s.asyncKeyPtrChan.Write(entry)
	s.pendingUpdates.Add(1)
}

func (s *Segment) Scan(startKey []byte, count int, snapshotTs uint64) []Pair[[]byte, y.ValueStruct] {
	delay := time.Duration(1)
	for s.pendingUpdates.Load() > 0 {
		// Waiting time was generally between 10-250ms
		time.Sleep(delay * time.Millisecond)
		delay = delay * 2
	}

	snapshotTsNano := snapshotTs

	// 1. Do range scan
	internalKey := y.KeyWithTs(startKey, snapshotTs)
	startRow := Pair[[]byte, *list.Element]{Key: internalKey}
	uniqueKVs := make(map[string]y.ValueStruct)
	seenKeys := make(map[string]any)

	idx := 1
	s.tree.Ascend(startRow, func(item Pair[[]byte, *list.Element]) bool {
		if idx > count {
			return false
		}

		// expiredTs < ItemTs < snapshotTs
		itemTs := y.ParseTs(item.Key)
		lessThanOrEqualToSnapshotTs := itemTs <= snapshotTsNano

		if lessThanOrEqualToSnapshotTs {
			strKey := string(y.ParseKey(item.Key))
			if _, seen := seenKeys[strKey]; !seen {
				seenKeys[strKey] = true
				if item.Val != nil {
					uniqueKVs[strKey] = item.Val.Value.(y.ValueStruct)
					idx++
				}
			}
		}
		return true
	})

	return MapToArray(uniqueKVs)
}

func (s *Segment) Free() int {
	//NOTE: DO NOT CLOSE WRITER THREAD HERE.
	removedCount := s.Len()

	s.tree.Clear()
	s.vlog.Init()

	return removedCount
}

func (s *Segment) Len() int {
	return s.tree.Len()
}

func (s *Segment) Put(key []byte, val y.ValueStruct) {
	rPtr := s.AddValue(val)
	entry := &Pair[[]byte, *list.Element]{Key: key, Val: rPtr}
	s.AddIndex(entry)
}

func (s *Segment) Empty() bool {
	return s.Len() == 0
}

func (s *Segment) NewIterator() y.Iterator {
	return &SegIterator{
		iter: s.tree.Iter(),
	}
}
