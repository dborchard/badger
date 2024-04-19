package segring

import (
	"bytes"
	"github.com/dgraph-io/badger/y"
	"sort"
	"time"
)

type Pair[K any, V any] struct {
	Key K
	Val V
}

// CompareKeys checks the key without timestamp and checks the timestamp if keyNoTs
// is same.
// a<timestamp> would be sorted higher than aa<timestamp> if we use bytes.compare
// All keys should have timestamp.
func CompareKeys(key1, key2 []byte) int {
	if cmp := bytes.Compare(key1[:len(key1)-8], key2[:len(key2)-8]); cmp != 0 {
		return cmp
	}
	return bytes.Compare(key1[len(key1)-8:], key2[len(key2)-8:])
}

func IsValidTsUint(ts uint64, ttl time.Duration) bool {
	lastValidTs := time.Now().Add(-1 * ttl)
	lastValidTsUint := ToUnit64(lastValidTs)

	return ts > lastValidTsUint
}

func ToUnit64(ts time.Time) uint64 {
	return uint64(ts.UnixNano())
}

func Uint64ToTime(ts uint64) time.Time {
	seconds := int64(ts / 1000000000)
	nanoseconds := int64(ts % 1000000000)
	currTs := time.Unix(seconds, nanoseconds)
	return currTs
}

func MapToArray(uniqueKVs map[string]y.ValueStruct) []Pair[[]byte, y.ValueStruct] {
	// 2. Sorted key set
	var keys []string
	for k := range uniqueKVs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < (keys[j])
	})

	// 3. Map -> Array
	var result []Pair[[]byte, y.ValueStruct]
	for _, k := range keys {
		pair := Pair[[]byte, y.ValueStruct]{Key: []byte(k), Val: uniqueKVs[k]}
		result = append(result, pair)
	}
	return result
}
