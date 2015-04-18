## hashq

[![GoDoc](https://godoc.org/github.com/z0rr0/hashq?status.svg)](https://godoc.org/github.com/z0rr0/hashq) [![LGPL License](http://img.shields.io/badge/license-LGPLv3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0.txt)

Go package to control incoming tasks that need to use some shared resources.

For example, it can be used if there are many incoming requests per second and every one should read some data from database, then it's inefficient to open/close a connection every time. So some shared connections pool can be allocated and used, but we should control it - safe open new and close unused element.

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