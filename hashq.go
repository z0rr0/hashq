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
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "sync"
    "time"
)

const (
    incomePeriodTotal int64 = 1000000 // 1 ms (nanoseconds)
)

var (
    // loggerError implements error logger.
    loggerError = log.New(os.Stderr, "ERROR [hashq]: ", log.Ldate|log.Ltime|log.Lshortfile)
    // loggerDebug implements debug logger, it's disabled by default.
    loggerDebug = log.New(ioutil.Discard, "DEBUG [hashq]: ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
)

// HashQ is a hash storage.
type HashQ struct {
    sharedMap   []*Resource   // an array of shared resources wrappers
    emptyPt     Shared        // an example of default Shared object
    maxShared   int64         // a number of shared resources
    cleanPeriod time.Duration // an interval between attempts to clean resources
    olderPeriod time.Duration // an interval after wich a resource can be considered obsolete
    curSpeed    *Speed        // a speed structure of incoming requests
}

// Len returns a length of hash storage.
func (hq *HashQ) Len() int {
    return len(hq.sharedMap)
}

// Shared is an interface of a shared resource.
type Shared interface {
    Open() (Shared, error)
    Close()
}

// Resource is a wrapper of shared resource to
// safe handle its operations.
type Resource struct {
    Pt       Shared
    parent   *HashQ
    active   bool
    open     sync.Once
    mutex    sync.RWMutex
    modified time.Time
}

// Speed is structure to calculate average speed and period of incoming requests.
// It uses probabilistic values, because this works without any locks.
type Speed struct {
    Sum          float64 // sum of incoming requests
    Start        int64   // mark of init time
    Last         int64   // mark of last request time
    incomePeriod int64   // init period of incoming requests (milliseconds)
    speedCheck   uint64  // it is parameter to check time interval between incoming requests
}

// Debug turns on debug mode.
func Debug(debug bool) {
    debugHandle := ioutil.Discard
    if debug {
        debugHandle = os.Stdout
    }
    loggerDebug = log.New(debugHandle, "DEBUG [hashq]: ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
}

// New initializes hash table storage.
//
// share - a empty (closed) shared resource
// size - a number of shared resources;
// spch - a parameter to check time interval between incoming requests;
// cleaner - an interval between attempts to clean resources;
// older - a resource can be considered obsolete after this interval.
func New(share Shared, size int64, spch uint64, cleaner, older time.Duration) (*HashQ, error) {
    var err error
    hq := &HashQ{}
    switch {
    case share == nil:
        err = fmt.Errorf("can't initializes using nil pointer")
    case size < 1:
        err = fmt.Errorf("bad size of shared array")
    case spch < 1:
        err = fmt.Errorf("bad parameter for period correction")
    }
    if err != nil {
        return hq, err
    }
    initTime := time.Now().UnixNano()
    shMap, speed := make([]*Resource, size), &Speed{0, initTime, initTime, incomePeriodTotal, spch}
    hq = &HashQ{shMap, share, size, cleaner, older, speed}
    for i := range hq.sharedMap {
        hq.sharedMap[i] = &Resource{}
        hq.sharedMap[i].parent = hq
        hq.sharedMap[i].Pt = hq.emptyPt
    }
    go func() {
        ticker := time.Tick(hq.cleanPeriod)
        for {
            select {
            case <-ticker:
                hq.Clean(false)
            }
        }
    }()
    return hq, nil
}

// Clean resets unused resources. Only this method can delete shared pointers.
func (hq *HashQ) Clean(forced bool) {
    loggerDebug.Printf("start Clean(forced=%v)", forced)
    defer loggerDebug.Printf("end Clean(forced=%v)", forced)

    for i, res := range hq.sharedMap {
        if (res.active) && (forced || (time.Now().Sub(res.modified) > hq.olderPeriod)) {
            loggerDebug.Printf("try to close %v\n", i)
            res.Clean(i)
        }
    }
}

// genHash returns a hash index for new incoming request.
func (hq *HashQ) genHash() int64 {
    hq.curSpeed.inc()
    if hq.curSpeed.Check() {
        if f := hq.curSpeed.Period(); f != 0 {
            hq.curSpeed.incomePeriod = f
            loggerDebug.Printf("incoming period was corrected: %v\n", f)
        }
    }
    idx := (time.Now().UnixNano() / hq.curSpeed.incomePeriod) % hq.maxShared
    loggerDebug.Printf("get idx=%v\n", idx)
    return idx
}

// Get returns a shared resource.
func (hq *HashQ) Get() (*Resource, error) {
    if (hq == nil) || (hq.Len() == 0) {
        return nil, fmt.Errorf("configuration is not initialized")
    }
    idx := hq.genHash()
    res := hq.sharedMap[idx]
    loggerDebug.Printf("Get() returned [%v]=%v", idx, &hq.sharedMap[idx])
    return res, nil
}

// Stat print statistics for debug mode.
func (hq *HashQ) Stat() {
    loggerDebug.Println("Stat")
    for i, res := range hq.sharedMap {
        loggerDebug.Printf("\t %v: pt=%v\n", i, res.Pt)
    }
}

// inc increments counters of Speed structure.
func (s *Speed) inc() {
    s.Sum, s.Last = s.Sum+1, time.Now().UnixNano()
}

// Period returns an average period of incoming requests
func (s *Speed) Period() int64 {
    // if s.Sum == 0 {
    //     return 0
    // }
    return (s.Last - s.Start) / int64(s.Sum)
}

// Check verifies that period should be corrected.
func (s *Speed) Check() bool {
    return (uint64(s.Sum) % s.speedCheck) == 1
}

// Clean resets an unused resource and "closes" shared object.
// Index i is not important here, and used only for debug.
func (res *Resource) Clean(i int) {
    res.mutex.Lock()
    defer res.mutex.Unlock()

    res.Pt.Close()
    res.Pt, res.open, res.active = res.parent.emptyPt, sync.Once{}, false
    res.touch()
    loggerDebug.Printf("%v %v is closed", i, &res)
    res.parent.Stat()
}

// Lock implements read-lock for a resource.
// If Lock was called then a resource can't be closed.
func (res *Resource) Lock() {
    res.touch()
    res.mutex.RLock()
}

// Unlock implements read-unlock for a resource.
func (res *Resource) Unlock() {
    res.touch()
    res.mutex.RUnlock()
}

// touch updates last modified time of a resource object.
func (res *Resource) touch() {
    res.modified = time.Now()
}

// TryOpen calls Open() method of shared resource only once.
// This method can be call only if the resource is locked.
func (res *Resource) TryOpen() (Shared, error) {
    var err error
    open := func() {
        loggerDebug.Println("called TryOpen()")
        res.Pt, err = res.Pt.Open()
        res.active = true
    }
    res.open.Do(open)
    return res.Pt, err
}
