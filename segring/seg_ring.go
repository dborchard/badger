package segring

import (
	"bytes"
	"container/list"
	"context"
	"github.com/dgraph-io/badger/y"
	"math"
	"runtime"
	"time"
)

type SegRing struct {
	segments []*Segment

	ttlValidSegmentsCount int

	segmentDuration time.Duration
	TTL             time.Duration
	cycleDuration   int64
}

func New(gcInterval, ttl time.Duration, ctx context.Context) *SegRing {
	// create SegmentRing instance
	sr := SegRing{segmentDuration: gcInterval}

	// calc copy-ahead segment count
	sr.ttlValidSegmentsCount = int(math.Ceil(float64(ttl) / float64(sr.segmentDuration)))

	// init with segments
	totalSegments := 3*sr.ttlValidSegmentsCount + 2
	sr.cycleDuration = int64(time.Duration(float64(totalSegments) * float64(gcInterval)).Seconds())
	sr.init(totalSegments, ctx)

	sr.TTL = ttl
	go sr.StartGc(gcInterval, ctx)

	return &sr
}

func (s *SegRing) init(size int, ctx context.Context) {
	s.segments = make([]*Segment, size)

	// Create segment array
	for i := 0; i < size; i++ {
		s.segments[i] = NewSegment(ctx)
	}

	// Create Ptr ring
	for i := 0; i < size-1; i++ {
		s.segments[i].nextPtr = s.segments[i+1]
	}
	s.segments[size-1].nextPtr = s.segments[0]

	// Start Listeners in all the Segments
	for i := 0; i < size; i++ {
		s.segments[i].StartListener()
	}
}

func (s *SegRing) Put(key []byte, val y.ValueStruct) {
	currTs := y.ParseTs(key)
	currTsTime := Uint64ToTime(y.ParseTs(key))

	activeSegmentIdx := s.findSegmentIdx(currTsTime)

	// 2. Add to Segment "VLOG"
	rPtr := s.segments[activeSegmentIdx].AddValue(val)

	// 3. Create entry for "Index"
	internalKey := y.KeyWithTs(key, currTs)
	entry := &Pair[[]byte, *list.Element]{Key: internalKey, Val: rPtr}

	// 4.a Add to Curr segment in sync.
	head := s.segments[activeSegmentIdx]
	head.AddIndex(entry)
	head = head.nextPtr

	// 4.b Parallel write to Curr+1 .... segments async.
	// Due to wait group in segment, we are guaranteed that Scans will see the written value.
	for i := 1; i <= s.ttlValidSegmentsCount; i++ {
		head.AddIndexAsync(entry)
		head = head.nextPtr
	}
}

func (s *SegRing) Get(key []byte) y.ValueStruct {

	readTs := y.ParseTs(key)
	res := s.Scan(key, 1, readTs)

	if len(res) != 1 || !bytes.Equal(res[0].Key, key) {
		return y.ValueStruct{}
	}
	return res[0].Val
}

func (s *SegRing) Scan(startKey []byte, count int, snapshotTs uint64) []Pair[[]byte, y.ValueStruct] {
	//0. Check if snapshotTs has already expired
	if !IsValidTsUint(snapshotTs, s.TTL) {
		return []Pair[[]byte, y.ValueStruct]{}
	}

	//1. Get Segment
	segmentIdx := s.findSegmentIdx(Uint64ToTime(snapshotTs))

	// 2. Range Scan delegation
	return s.segments[segmentIdx].Scan(startKey, count, snapshotTs)
}

func (s *SegRing) Empty() bool {
	return s.GetCurrSegment().Len() == 0
}

func (s *SegRing) GetLastSegment() *Segment {
	currTsTime := time.Now().Add(-s.segmentDuration)
	activeSegmentIdx := s.findSegmentIdx(currTsTime)

	return s.segments[activeSegmentIdx]
}

func (s *SegRing) GetCurrSegment() *Segment {
	currTsTime := time.Now()
	activeSegmentIdx := s.findSegmentIdx(currTsTime)

	return s.segments[activeSegmentIdx]
}

func (s *SegRing) Close() {
	for _, segment := range s.segments {
		segment.Free()
	}
}

func (s *SegRing) StartGc(interval time.Duration, ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.Prune()
		case <-ctx.Done():
			return
		}
	}
}

func (s *SegRing) Prune() int {
	// ref call.
	currSegmentIdx := s.findSegmentIdx(time.Now())
	pruneSegmentIdx := currSegmentIdx - 1 - (2 * s.ttlValidSegmentsCount)
	if pruneSegmentIdx < 0 {
		pruneSegmentIdx += len(s.segments)
	}
	deleteCount := s.segments[pruneSegmentIdx].Free()
	runtime.GC()

	return deleteCount
}

func (s *SegRing) findSegmentIdx(snapshotTs time.Time) int {
	cycleOffset := float64(snapshotTs.Unix() % s.cycleDuration)
	segmentIdx := int(math.Floor(cycleOffset / s.segmentDuration.Seconds()))
	return segmentIdx
}
