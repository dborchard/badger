/*
 * Copyright 2017 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package badger

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dgraph-io/badger/y"
)

func TestValueBasic(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	y.Check(err)

	kv := NewKV(getTestOptions(dir))
	defer kv.Close()
	log := &kv.vlog

	entry := &Entry{
		Key:             []byte("samplekey"),
		Value:           []byte("sampleval"),
		Meta:            BitValuePointer,
		CASCounterCheck: 22222,
		casCounter:      33333,
	}
	entry2 := &Entry{
		Key:             []byte("samplekeyb"),
		Value:           []byte("samplevalb"),
		Meta:            BitValuePointer,
		CASCounterCheck: 22225,
		casCounter:      33335,
	}

	b := new(request)
	b.Entries = []*Entry{entry, entry2}

	log.Write([]*request{b})
	require.Len(t, b.Ptrs, 2)
	fmt.Printf("Pointer written: %+v %+v", b.Ptrs[0], b.Ptrs[1])

	e, err := log.Read(b.Ptrs[0], nil)
	e2, err := log.Read(b.Ptrs[1], nil)

	require.NoError(t, err)
	readEntries := []Entry{e, e2}
	require.EqualValues(t, []Entry{
		{
			Key:             []byte("samplekey"),
			Value:           []byte("sampleval"),
			Meta:            BitValuePointer,
			CASCounterCheck: 22222,
			casCounter:      33333,
		},
		{
			Key:             []byte("samplekeyb"),
			Value:           []byte("samplevalb"),
			Meta:            BitValuePointer,
			CASCounterCheck: 22225,
			casCounter:      33335,
		},
	}, readEntries)
}

func TestCompression(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	y.Check(err)

	opt := getTestOptions(dir)
	opt.ValueCompressionMinSize = 16

	kv := NewKV(opt)
	defer kv.Close()
	log := &kv.vlog

	entry := &Entry{
		Key:             []byte("key1"),
		Value:           []byte("shortval"),
		Meta:            BitValuePointer,
		CASCounterCheck: 22222,
		casCounter:      33333,
	}
	entry2 := &Entry{ // This entry will be compressed
		Key:             []byte("aaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Value:           []byte("aaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Meta:            BitValuePointer,
		CASCounterCheck: 22225,
		casCounter:      33335,
	}
	entry3 := &Entry{
		Key:             []byte("highentropy"),
		Value:           []byte("uncompressable"),
		Meta:            BitValuePointer,
		CASCounterCheck: 22226,
		casCounter:      33336,
	}

	b := new(request)
	b.Entries = []*Entry{entry, entry2, entry3}

	log.Write([]*request{b})
	require.Len(t, b.Ptrs, 3)
	fmt.Printf("Pointer written: %+v %+v %+v", b.Ptrs[0], b.Ptrs[1], b.Ptrs[2])

	e, err := log.Read(b.Ptrs[0], nil)
	e2, err := log.Read(b.Ptrs[1], nil)
	require.True(t, e2.Meta&BitCompressed > 0)
	entry2.Meta = entry2.Meta | BitCompressed
	e3, err := log.Read(b.Ptrs[2], nil)

	require.NoError(t, err)
	readEntries := []Entry{e, e2, e3}
	require.EqualValues(t, []Entry{
		*entry,
		*entry2,
		*entry3,
	}, readEntries)
}

func TestValueGC(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "badger")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	kv := NewKV(getTestOptions(dir))
	defer kv.Close()

	sz := 20 << 20
	var entries []*Entry
	for i := 0; i < 100; i++ {
		v := make([]byte, sz)
		rand.Read(v[:rand.Intn(sz)]) // some values can be compressed
		entries = append(entries, &Entry{
			Key:   []byte(fmt.Sprintf("key%d", i)),
			Value: v,
		})
	}
	kv.BatchSet(entries)
	for _, e := range entries {
		require.NoError(t, e.Error, "entry with error: %+v", e)
	}

	for i := 0; i < 45; i++ {
		kv.Delete([]byte(fmt.Sprintf("key%d", i)))
	}

	kv.vlog.RLock()
	lf := kv.vlog.files[0]
	kv.vlog.RUnlock()

	//	lf.iterate(0, func(e Entry) bool {
	//		e.print("lf")
	//		return true
	//	})

	kv.vlog.rewrite(lf)
	for i := 45; i < 100; i++ {
		val, _ := kv.Get([]byte(fmt.Sprintf("key%d", i)))
		require.NotNil(t, val)
		require.True(t, len(val) == sz, "Size found: %d", len(val))
	}
}

func BenchmarkReadWrite(b *testing.B) {
	rwRatio := []float32{
		0.1, 0.2, 0.5, 1.0,
	}
	valueSize := []int{
		64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384,
	}

	for _, vsz := range valueSize {
		for _, rw := range rwRatio {
			b.Run(fmt.Sprintf("%3.1f,%04d", rw, vsz), func(b *testing.B) {
				var vl valueLog
				vl.Open(nil, getTestOptions("vlog"))
				defer os.Remove("vlog")
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					e := new(Entry)
					e.Key = make([]byte, 16)
					e.Value = make([]byte, vsz)
					bl := new(request)
					bl.Entries = []*Entry{e}

					var ptrs []valuePointer

					vl.Write([]*request{bl})
					ptrs = append(ptrs, bl.Ptrs...)

					f := rand.Float32()
					if f < rw {
						vl.Write([]*request{bl})
						ptrs = append(ptrs, bl.Ptrs...)

					} else {
						ln := len(ptrs)
						if ln == 0 {
							b.Fatalf("Zero length of ptrs")
						}
						idx := rand.Intn(ln)
						e, err := vl.Read(ptrs[idx], nil)
						if err != nil {
							b.Fatalf("Benchmark Read: %v", err)
						}
						if len(e.Key) != 16 {
							b.Fatalf("Key is invalid")
						}
						if len(e.Value) != vsz {
							b.Fatalf("Value is invalid")
						}
					}
				}
			})
		}
	}
}
