## hashq

[![GoDoc](https://godoc.org/github.com/z0rr0/hashq?status.svg)](https://godoc.org/github.com/z0rr0/hashq) [![LGPL License](http://img.shields.io/badge/license-LGPLv3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0.txt) [![Build Status](https://travis-ci.org/z0rr0/hashq.svg?branch=master)](https://travis-ci.org/z0rr0/hashq)

Go package to control incoming tasks that need to use some shared resources.

It contains a storage for some resources. Unused elements will be closed automatically after needed time.

For example, it can be used if there are many incoming requests and every one should read some data from database. So, some shared connections pool can be allocated and used, and we shouldn't control it, **hashq** will store a set and return needed elements using round robin algorithm.

Version 2.0 is not backwards compatible with previous ones. There are following main changes:

* element's interface is changed, it contains methods Close() and CanClose()
* new HashQ constructor with 3 parameters
* the pool doesn't control custom locks/unlocks operation and works only as a storage
* the pool doesn't use time hashing now, the round robing algorithm is used

```go
var (
    poolSize int64 = 128              // storage size
    cleanPeriod    = 90 * time.Second // Clean() function will be called after this period
    waitAfterClose = 1 * time.Second  // wait this period after connection close
)

type Connection struct {
	// some fields
}
// to initialize the pool by initial values
// often, it empty structure or its pointer
func (con *Connection) New() Shared {...}
// CanClose can return always true if it isn't needed
// it is only to skip used elements without locks.
func (con *Connection) CanClose() bool {...}
// Close should contain all needed locks/unlocks.
func (con *Connection) Close() {...}

// create new pool
pool := New(poolSzie, &Connection{}, 0)
ch, errc := make(chan Shared), make(chan error)
// the pool will send new elements to channel ch
go pool.Produce(ch, ec)
// check that Producer is started without any errors
if err := <-errc; err == nil {
    panic(err)
}
// Monitor can be periodically called to run CanClose+Close
// for each element in the pool
go pool.Monitor(cleanPeriod)
// get element
sh := <-ch
conn := sh.(*Connection)
// use conn...
```

### Dependencies

Standard [Go library](http://golang.org/pkg/).

### Design guidelines

There are recommended style guides:

* [The Go Programming Language Specification](https://golang.org/ref/spec)
* [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

[Go-lint](http://go-lint.appspot.com/github.com/z0rr0/hashq) tool.

### Testing

Standard Go testing way:

```shell
cd $GOPATH/src/github.com/z0rr0/hashq
go test -v -cover
```

---

*This source code is governed by a [LGPLv3](https://www.gnu.org/licenses/lgpl-3.0.txt) license that can be found in the [LICENSE](https://github.com/z0rr0/hashq/blob/master/LICENSE) file.*

<img src="https://www.gnu.org/graphics/lgplv3-147x51.png" title="LGPLv3 logo">