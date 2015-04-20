## hashq

[![GoDoc](https://godoc.org/github.com/z0rr0/hashq?status.svg)](https://godoc.org/github.com/z0rr0/hashq) [![LGPL License](http://img.shields.io/badge/license-LGPLv3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0.txt) [![Build Status](https://travis-ci.org/z0rr0/hashq.svg?branch=master)](https://travis-ci.org/z0rr0/hashq)

Go package to control incoming tasks that need to use some shared resources.

It contains a storage for some resources that can be opened and closed to don't call these procedures every time. An opened item will not be closed immediately, so it can be used for new calls. Unused elements will be closed automatically after needed time.

For example, it can be used if there are many incoming requests and every one should read some data from database, then it's inefficient to open/close a connection every time. So, some shared connections pool can be allocated and used, and we shouldn't control it, **hashq** will do it - thread safe open and close calls.

```go
var (
    sharedNum int64 = 16           // storage size
    checkFreq uint64 = 10          // recheck a frequency every checkFreq calls
    cleanerTime = time.Second * 30 // Clean() function will be called after this period
    olderTime = time.Second * 10   // connection can be closed after this period
)

type Connection struct {
	// some fields
}
func (con *Connection) Open() (Shared, error) {
	// ...
}
func (con *Connection) Close(){
	// ...
}

empty := &Connection{}
if err := Init(empty, sharedNum, checkFreq, cleanerTime, olderTime); err == nil {
    panic("incorrect initialization")
}

// get an item from storage and open it if it's needed
res, err := Get()
if err != nil {
    panic("can't get element")  // any error handling can be used instead panic()
}
res.Lock()                      // any other goroutine can call Lock()
defer res.Unlock()              // Clean() can check an close "old" unlocked items
sharedCon, err := res.TryOpen() // Open() will be called only once
if err != nil {
    panic("can't open resource")
}
con := sharedCon.(*Connection)
// use gotten Connection element
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