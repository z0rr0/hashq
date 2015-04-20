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

var (
    // SharedMap is an array of shared resources wrappers.
    SharedMap []*Resource

    initialized bool
    // emptyPt is an example of default Shared object
    emptyPt Shared
    // cleanPeriod is an interval between attempts to clean resources.
    cleanPeriod = time.Second * 60
    // olderPeriod is an interval after wich a resource can be considered obsolete.
    olderPeriod = time.Second * 30
    // maxShared is a number of shared resources.
    maxShared int64 = 4
    // speedCheck is parameter to check time interval between incoming requests.
    speedCheck uint64 = 10
    // curSpeed is a speed structure of incoming requests.
    curSpeed *Speed
    // incomeFreq is initialize value of incoming requests.
    // It will be corrected after the first one.
    incomeFreq int64 = 1000000 // milliseconds
    // loggerError implements error logger.
    loggerError = log.New(os.Stderr, "ERROR [hashq]: ", log.Ldate|log.Ltime|log.Lshortfile)
    // loggerDebug implements debug logger, it's disabled by default.
    loggerDebug = log.New(ioutil.Discard, "DEBUG [hashq]: ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
)

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
    Sum   float64 // sum of incoming requests
    Start int64   // mark of init time
    Last  int64   // mark of last request time
}

// Debug turns on debug mode.
func Debug(debug bool) {
    debugHandle := ioutil.Discard
    if debug {
        debugHandle = os.Stdout
    }
    loggerDebug = log.New(debugHandle, "DEBUG [hashq]: ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
}

// Init initializes shared map:
//
// size - a number of shared resources;
// spch - a parameter to check time interval between incoming requests;
// cleaner - an interval between attempts to clean resources;
// older - a resource can be considered obsolete after this interval.
func Init(share Shared, size int64, spch uint64, cleaner, older time.Duration) error {
    var err error
    switch {
        case share == nil:
            err = fmt.Errorf("can't initializes using nil pointer")
        case size < 1:
            err = fmt.Errorf("bad size of shared array")
        case spch < 1:
            err = fmt.Errorf("bad parameter for frequency correction")
    }
    if err != nil {
        return err
    }
    emptyPt = share
    maxShared, speedCheck, cleanPeriod, olderPeriod = size, spch, cleaner, older
    SharedMap = make([]*Resource, maxShared)
    for i := range SharedMap {
        SharedMap[i] = &Resource{}
        SharedMap[i].Pt = emptyPt
    }
    go func() {
        ticker := time.Tick(cleanPeriod)
        for {
            select {
                case <-ticker:
                    Clean()
            }
        }
    }()
    InitSpeed()
    initialized = true
    return err
}

// InitSpeed initializes new internal Speed structure.
// This method should be called before the first requests.
func InitSpeed() {
    initTime := time.Now().UnixNano()
    curSpeed = &Speed{0, initTime, initTime}
}

// Clean resets unused resources. Only this method can delete shared pointers.
func Clean() {
    if !initialized {
        return
    }
    loggerDebug.Println("start Clean()")
    defer loggerDebug.Println("end Clean()")

    for i, res := range SharedMap {
        if (res.active) && (time.Now().Sub(res.modified) > olderPeriod) {
            loggerDebug.Printf("try to close %v\n", i)
            res.Clean(i)
        }
    }
}

// genHash returns a hash index for new incoming request.
func genHash() int64 {
    curSpeed.inc()
    if curSpeed.Check() {
        if f := curSpeed.Freq(); f != 0 {
            incomeFreq = f
            loggerDebug.Printf("incomeFreq was corrected: %v\n", f)
        }
    }
    idx := (time.Now().UnixNano() / incomeFreq) % maxShared
    loggerDebug.Printf("get idx=%v\n", idx)
    return idx
}

// Get returns a shared resource.
func Get() (*Resource, error) {
    if !initialized {
        return nil, fmt.Errorf("configuration is not initialized")
    }
    idx := genHash()
    res := SharedMap[idx]
    loggerDebug.Printf("Get() returned [%v]=%v", idx, &SharedMap[idx])
    return res, nil
}

// Stat print statistics for debug mode.
func Stat() {
    loggerDebug.Println("Stat")
    for i, res := range SharedMap {
        loggerDebug.Printf("\t %v: pt=%v\n", i, res.Pt)
    }
}

// inc increments counters of Speed structure.
func (s *Speed) inc() {
    s.Sum, s.Last = s.Sum + 1, time.Now().UnixNano()
}

// Freq returns an average frequency of incoming requests
func (s *Speed) Freq() int64 {
    if s.Sum == 0 {
        return 0
    }
    return (s.Last - s.Start) / int64(s.Sum)
}
// Check verifies that frequency should be corrected.
func (s *Speed) Check() bool {
    return (uint64(s.Sum) % speedCheck) == 1
}

// Clean resets an unused resource and "closes" shared object.
func (res *Resource) Clean(i int) {
    res.mutex.Lock()
    defer res.mutex.Unlock()

    res.Pt.Close()
    res.Pt, res.open, res.active = emptyPt, sync.Once{}, false
    res.touch()
    loggerDebug.Printf("%v is closed\n", i)
    Stat()
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
