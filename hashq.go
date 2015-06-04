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
    incomeFreqTotal int64 = 1000000 // milliseconds
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
    active   bool
    open     sync.Once
    mutex    sync.RWMutex
    modified time.Time
}

// Speed is structure to calculate average speed and frequency of incoming requests.
// It uses probabilistic values, because this works without any locks.
type Speed struct {
    Sum        float64 // sum of incoming requests
    Start      int64   // mark of init time
    Last       int64   // mark of last request time
    incomeFreq int64   // init frequency of incoming requests (milliseconds)
    speedCheck uint64  // it is parameter to check time interval between incoming requests
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
        err = fmt.Errorf("bad parameter for frequency correction")
    }
    if err != nil {
        return hq, err
    }
    initTime := time.Now().UnixNano()
    shMap, speed := make([]*Resource, size), &Speed{0, initTime, initTime, incomeFreqTotal, spch}
    hq = &HashQ{shMap, share, size, cleaner, older, speed}
    for i := range hq.sharedMap {
        hq.sharedMap[i] = &Resource{}
        hq.sharedMap[i].Pt = hq.emptyPt
    }
    go func() {
        ticker := time.Tick(hq.cleanPeriod)
        for {
            select {
            case <-ticker:
                hq.Clean()
            }
        }
    }()
    return hq, nil
}

// Clean resets unused resources. Only this method can delete shared pointers.
func (hq *HashQ) Clean() {
    loggerDebug.Println("start Clean()")
    defer loggerDebug.Println("end Clean()")

    for i, res := range hq.sharedMap {
        if (res.active) && (time.Now().Sub(res.modified) > hq.olderPeriod) {
            loggerDebug.Printf("try to close %v\n", i)
            res.Clean(hq, i)
        }
    }
}

// genHash returns a hash index for new incoming request.
func (hq *HashQ) genHash() int64 {
    hq.curSpeed.inc()
    if hq.curSpeed.Check() {
        if f := hq.curSpeed.Freq(); f != 0 {
            hq.curSpeed.incomeFreq = f
            loggerDebug.Printf("incomeFreq was corrected: %v\n", f)
        }
    }
    idx := (time.Now().UnixNano() / hq.curSpeed.incomeFreq) % hq.maxShared
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

// Freq returns an average frequency of incoming requests
func (s *Speed) Freq() int64 {
    // if s.Sum == 0 {
    //     return 0
    // }
    return (s.Last - s.Start) / int64(s.Sum)
}

// Check verifies that frequency should be corrected.
func (s *Speed) Check() bool {
    return (uint64(s.Sum) % s.speedCheck) == 1
}

// Clean resets an unused resource and "closes" shared object.
func (res *Resource) Clean(hq *HashQ, i int) {
    res.mutex.Lock()
    defer res.mutex.Unlock()

    res.Pt.Close()
    res.Pt, res.open, res.active = hq.emptyPt, sync.Once{}, false
    res.touch()
    loggerDebug.Printf("%v is closed\n", i)
    hq.Stat()
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
