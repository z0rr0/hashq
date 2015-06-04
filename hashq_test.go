// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a LGPL-style
// license that can be found in the LICENSE file.

package hashq

import (
    "fmt"
    "math/rand"
    "testing"
    "time"
)

var (
    sharedNum   int64  = 4
    checkFreq   uint64 = 5
    cleanerTime        = time.Millisecond * 30
    olderTime          = time.Millisecond * 20

    minOpenTime       = time.Millisecond * 10
    maxOpenTime int64 = 5

    maxOpenValue int64 = 100
    waitError    bool

    minReqTime       = time.Millisecond * 5
    maxReqTime int64 = 30

    minRequestDelay       = time.Millisecond * 5
    maxRequestDelay int64 = 5

    maxRequests = 100
)

type Connection struct {
    ID     int64
    Active bool
}
type Request struct {
    ID      int64
    Con     *Connection
    Created time.Time
}
type Result struct {
    ID     int64
    Answer int64
    Err    error
}

func (con *Connection) Open() (Shared, error) {
    rand.Seed(time.Now().UnixNano())
    salt := time.Duration(rand.Int63n(maxOpenTime)) * time.Millisecond
    time.Sleep(minOpenTime + salt)

    c := &Connection{rand.Int63n(maxOpenValue) + 1, true}
    if waitError {
        return c, fmt.Errorf("expected error")
    }
    return c, nil
}
func (con *Connection) Close() {
    con.ID = 0
    con.Active = false
}
func (req *Request) Run(t *testing.T, hq *HashQ, result chan Result) {
    rand.Seed(time.Now().UnixNano())
    // it is some shared resource
    res, err := hq.Get()
    if err != nil {
        t.Errorf("wrong Get() response")
    }
    res.Lock()
    loggerDebug.Printf("run start for %v\n", &res)
    defer func() {
        loggerDebug.Printf("run end for %v\n", &res)
        res.Unlock()
    }()

    sharedCon, err := res.TryOpen()
    if err != nil {
        result <- Result{0, 0, err}
        return
    }
    con := sharedCon.(*Connection)
    if !con.Active {
        t.Errorf("inactive connection was used")
    }
    salt := time.Duration(rand.Int63n(maxReqTime)) * time.Millisecond
    time.Sleep(minReqTime + salt)
    // loggerDebug.Printf("send to channel: %v-%v", req.ID, con.ID)
    result <- Result{req.ID, req.ID * con.ID, nil}
}
func ReqGenerator(req chan Request) {
    rand.Seed(time.Now().UnixNano())
    reqNum := rand.Int63n(256)
    for i := 0; i < maxRequests; i++ {
        req <- Request{reqNum, &Connection{}, time.Now()}
        reqNum++
        salt := time.Duration(rand.Int63n(maxRequestDelay)) * time.Millisecond
        time.Sleep(minRequestDelay + salt)
    }
    close(req)
}

func TestDebug(t *testing.T) {
    if (loggerError == nil) || (loggerDebug == nil) {
        t.Errorf("incorrect references")
    }
    Debug(false)
    if (loggerError.Prefix() != "ERROR [hashq]: ") || (loggerDebug.Prefix() != "DEBUG [hashq]: ") {
        t.Errorf("incorrect loggers settings")
    }
    Debug(true)
    if (loggerError.Flags() != 19) || (loggerDebug.Flags() != 21) {
        t.Errorf("incorrect loggers settings")
    }
}

func TestGet(t *testing.T) {
    hq := &HashQ{}
    if _, err := hq.Get(); err == nil {
        t.Errorf("accept not initialized Get()")
    }
    hq.Clean()
}

func TestInit(t *testing.T) {
    var err error
    Debug(true)
    hq, empty := &HashQ{}, &Connection{}
    if _, err = New(nil, sharedNum, checkFreq, cleanerTime, olderTime); err == nil {
        t.Errorf("accept wrong parameters #1")
    }
    if _, err = New(empty, 0, checkFreq, cleanerTime, olderTime); err == nil {
        t.Errorf("accept wrong parameters #2")
    }
    if _, err = New(empty, sharedNum, 0, cleanerTime, olderTime); err == nil {
        t.Errorf("accept wrong parameters #3")
    }
    if hq, err = New(empty, sharedNum, checkFreq, cleanerTime, olderTime); err != nil {
        t.Errorf("incorrect initialization")
    }

    finish := make(chan bool)
    resultCh := make(chan Result)
    reqCh := make(chan Request)

    go func() {
        i := 0
        for result := range resultCh {
            // loggerDebug.Printf("result gotten: %v", result)
            if result.Err != nil {
                t.Errorf("result error: %v", result.Err)
            }
            if result.Answer == 0 {
                t.Errorf("incorrect answer: %v", result)
            }
            i++
            if i >= maxRequests {
                finish <- true
                return
            }
        }
    }()

    go ReqGenerator(reqCh)
    for req := range reqCh {
        reqpt := &req
        go reqpt.Run(t, hq, resultCh)
    }

    done := <-finish
    close(resultCh)
    time.Sleep(cleanerTime)
    t.Log("all done", done)
}
