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
    sharedNum int64 = 4
    checkFreq uint64 = 5
    cleanerTime = time.Millisecond * 30
    olderTime = time.Millisecond * 20

    minOpenTime = time.Millisecond * 10
    maxOpenTime int64 = 5

    maxOpenValue int64 = 100
    waitError bool

    minReqTime = time.Millisecond * 5
    maxReqTime int64 = 30

    minRequestDelay = time.Millisecond * 5
    maxRequestDelay int64 = 5

    maxRequests = 50
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
    Answer int64
    Err    error
}

func (con *Connection) Open() (Shared, error) {
    rand.Seed(time.Now().UnixNano())
    salt := time.Duration(rand.Int63n(maxOpenTime)) * time.Millisecond
    time.Sleep(minOpenTime + salt)

    c := &Connection{rand.Int63n(maxOpenValue), true}
    if waitError {
        return c, fmt.Errorf("expected error")
    }
    return c, nil
}
func (con *Connection) Close() {
    con.ID = 0
    con.Active = false
}
func (req *Request) Run(t *testing.T, result chan Result) {
    rand.Seed(time.Now().UnixNano())
    // it is some shared resource
    res := Get()
    res.Lock()
    loggerDebug.Printf("run start for %v\n", &res)
    defer func() {
        loggerDebug.Printf("run end for %v\n", &res)
        res.Unlock()
    }()

    sharedCon, err := res.TryOpen()
    if err != nil {
        result<-Result{0, err}
        return
    }
    con := sharedCon.(*Connection)
    if !con.Active {
        t.Errorf("inactive connection was used")
    }
    salt := time.Duration(rand.Int63n(maxReqTime)) * time.Millisecond
    time.Sleep(minReqTime + salt)
    result<-Result{req.ID * con.ID, nil}
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

func TestInit(t *testing.T) {
    Debug(true)
    empty := &Connection{}
    Init(empty, sharedNum, checkFreq, cleanerTime, olderTime)

    finish := make(chan bool)
    resultCh := make(chan Result)
    reqCh := make(chan Request)

    go func() {
        i := 0
        for result := range resultCh {
            if result.Err != nil {
                t.Errorf("result error: %v", result.Err)
            }
            if result.Answer == 0 {
                t.Errorf("incorrect answer")
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
        go reqpt.Run(t, resultCh)
    }

    done := <-finish
    close(resultCh)
    t.Log("all done", done)
}
