# Badger [![GoDoc](https://godoc.org/github.com/dgraph-io/badger?status.svg)](https://godoc.org/github.com/dgraph-io/badger)

An embeddable, persistent, simple and fast key-value (KV) store, written natively in Go.

![Badger sketch](/images/sketch.jpg)

## About

Badger is written out of frustration with existing KV stores which are either natively written in Go and slow, or fast but require usage of Cgo.
Badger aims to provide an equal or better speed compared to industry leading KV stores (like RocksDB), while maintaining the entire code base in Go natively.

## Design Goals

Badger has these design goals in mind:

- Write it natively in Go.
- Use latest research to build the fastest KV store for data sets spanning terabytes.
- Keep it simple, stupid. No support for transactions, versioning or snapshots -- anything that can be done outside of the store should be done outside.
- Optimize for SSDs (more below).

### Non-Goals

- Try to be a database.

## Design

Badger is based on [WiscKey paper by University of Wisconsin, Madison](https://www.usenix.org/system/files/conference/fast16/fast16-papers-lu.pdf).

In simplest terms, keys are stored in LSM trees along with pointers to values, which would be stored in write-ahead log files, aka value logs.
Keys would typically be kept in RAM, values would be served directly off SSD.

### Optimizations for SSD

SSDs are best at doing serial writes (like HDD) and random reads (unlike HDD).
Each write reduces the lifecycle of an SSD, so Badger aims to reduce the write amplification of a typical LSM tree.

It achieves this by separating the keys from values. Keys tend to be smaller in size and are stored in the LSM tree.
Values (which tend to be larger in size) are stored in value logs, which also double as write ahead logs for fault tolerance.

Only a pointer to the value is stored along with the key, which significantly reduces the size of each KV pair in LSM tree.
This allows storing lot more KV pairs per table. For e.g., a table of size 64MB can store 2 million KV pairs assuming an average key size of 16 bytes, and a value pointer of 16 bytes (with prefix diffing in Badger, the average key sizes stored in a table would be lower).
Thus, lesser compactions are required to achieve stability for the LSM tree, which results in fewer writes (all writes being serial).

### Nature of LSM trees

Because only keys (and value pointers) are stored in LSM tree, Badger generates much smaller LSM trees.
Even for huge datasets, these smaller trees can fit nicely in RAM allowing for lot quicker accesses to and iteration through keys.
For random gets, keys can be quickly looked up from within RAM, giving access to the value pointer.
Then only a single pointed read from SSD (random read) is done to retrieve value.
This improves random get performance significantly compared to traditional LSM tree design used by other KV stores.

## Comparisons

| Feature             | Badger                                       | RocksDB            | BoltDB    |
| -------             | ------                                       | -------            | ------    |
| Design              | LSM tree with value log                      | LSM tree only      | B+ tree   |
| High RW Performance | Yes                                          | Yes                | No        |
| Designed for SSDs   | Yes (with latest research <sup>1</sup>)                  | Not specifically <sup>2</sup> | No        |
| Embeddable          | Yes                                          | Yes                | Yes       |
| Sorted KV access    | Yes                                          | Yes                | Yes       |
| Go Native (no Cgo)  | Yes                                          | No                 | Yes       |
| Transactions        | No (but provides compare and set operations) | Yes (but non-ACID) | Yes, ACID |
| Snapshots           | No                                           | Yes                | Yes       |

<sup>1</sup> Badger is based on a paper called [WiscKey by University of Wisconsin, Madison](https://www.usenix.org/system/files/conference/fast16/fast16-papers-lu.pdf), which saw big wins with separating values from keys, significantly reducing the write amplification compared to a typical LSM tree.

<sup>2</sup> RocksDB is an SSD optimized version of LevelDB, which was designed specifically for rotating disks.
As such RocksDB's design isn't aimed at SSDs.

## Crash Consistency

Badger is crash resistent. Any update which was applied successfully before a crash, would be available after the crash.
Badger achieves this via its value log.

Badger's value log is a write-ahead log (WAL). Every update to Badger is written to this log first, before being applied to the LSM tree.
Badger maintains a monotonically increasing pointer (head) in the LSM tree, pointing to the last update offset in the value log.
As and when LSM table is persisted, the head gets persisted along with.
Thus, the head always points to the latest persisted offset in the value log.
Every time Badger opens the directory, it would first replay the updates after the head in order, bringing the updates back into the LSM tree; before it allows any reads or writes.
This technique ensures data persistence in face of crashes.

Furthermore, Badger can be run with `SyncWrites` option, which would open the WAL with O_DSYNC flag, hence syncing the writes to disk on every write.
