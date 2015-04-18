// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a LGPL-style
// license that can be found in the LICENSE file.

// Package hashq implements methods to control
// incoming tasks that need to use some shared resources.
//
package hashq

import (
    "io/ioutil"
    "log"
    "os"
    "sync"
    "time"
)

var (
    // MaxShared is a number of shared resources.
    MaxShared int64 = 4
    // SpeedCheck is parameter to check time interval of incoming requests.
    SpeedCheck uint64 = 10

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
    Open() (*interface{}, error)
    Close()
}

// Resource is a wrapper of shared resource to
// safe handle its operations.
type Resource struct {
    Res      *Shared
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

// InitSpeed initializes new internal Speed structure.
// This method should be called before the first requests.
func InitSpeed() {
    initTime := time.Now().UnixNano()
    curSpeed = &Speed{0, initTime, initTime}
}
// inc increments counters of Speed structure.
func (s *Speed) inc() {
    s.Sum, s.Last = s.Sum + 1, time.Now().UnixNano()
}
// Avg returns an average speed of incoming requests.
func (s *Speed) Avg() float64 {
    if s.Last == s.Start {
        return 0
    }
    return s.Sum / float64(s.Last - s.Start)
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
    return (uint64(s.Sum) % SpeedCheck) == 1
}
