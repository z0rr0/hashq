// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a LGPL-style
// license that can be found in the LICENSE file.

// Package hashq implements methods to control
// incoming tasks that need to use some shared resources.
//
// It contains a storage for some resources that can be opened and closed
// to don't call these procedures every time. An opened item will not be
// closed immediately, so it can be used for new calls.
// Unused elements will be closed automatically after needed time.
package hashq

import (
    "errors"
    "io/ioutil"
    "log"
    "os"
    "sync"
    "time"
)

var (
    // loggerError implements error logger.
    loggerError = log.New(os.Stderr, "ERROR [hashq]: ", log.Ldate|log.Ltime|log.Lshortfile)
    // loggerDebug implements debug logger, it's disabled by default.
    loggerDebug = log.New(ioutil.Discard, "DEBUG [hashq]: ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
)

// Shared is an interface of a shared resource.
type Shared interface {
    CanClose() bool
    Close(d time.Duration)
}

// HashQ is a hash storage.
type HashQ struct {
    pool      []Shared
    empty     Shared
    closeWait time.Duration
    mutex     sync.RWMutex
}

// Debug turns on debug mode.
func Debug(debug bool) {
    debugHandle := ioutil.Discard
    if debug {
        debugHandle = os.Stdout
    }
    loggerDebug = log.New(debugHandle, "DEBUG [hashq]: ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
}

func New(size int, e Shared, d time.Duration, debug bool) *HashQ {
    h := &HashQ{empty: e, closeWait: d}
    h.mutex.Lock()
    defer h.mutex.Unlock()
    // create a pool with initial elements
    for i := 0; i < size; i++ {
        h.pool = append(h.pool, h.empty)
    }
    Debug(debug)
    loggerDebug.Printf("new created a pool [%v]", size)
    return h
}

// Size returns a size of the pool.
func (h *HashQ) Size() int {
    h.mutex.RLock()
    defer h.mutex.RUnlock()
    return len(h.pool)
}

// Produce constantly scans the pool and writes Shared element to the channel sch.
// The channel can be buffered to exclude a bottle neck here.
func (h *HashQ) Produce(sch chan<- Shared, errch chan error) {
    if h.Size() == 0 {
        loggerError.Println("pool is empty")
        errch <- errors.New("empty pool")
        return
    }
    errch <- nil
    for {
        for _, s := range h.pool {
            sch <- s
        }
    }
}

// Monitor closes unused connections with period d.
func (h *HashQ) Monitor(d time.Duration) {
    clean := func() {
        loggerDebug.Printf("run connection clean, size=~%v", h.Size())
        h.mutex.RLock()
        defer h.mutex.RUnlock()
        i := 0
        for _, s := range h.pool {
            if s.CanClose() {
                s.Close(h.closeWait)
            }
        }
        loggerDebug.Printf("end connection clean, num=%v", i)
    }
    for {
        select {
        case <-time.After(d):
            clean()
        }
    }
}
