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
    ExpErr bool
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

func (c *Connection) Ping() error {
    rand.Seed(time.Now().UnixNano())
    if r := rand.Intn(2); r == 0 {
        // 50 %
        return fmt.Errorf("random error")
    }
    return nil
}

func (req *Request) Run(t *testing.T, hq *HashQ, result chan Result) {
    const retries = 3
    var (
        con       *Connection
        sharedCon Shared
    )
    rand.Seed(time.Now().UnixNano())
    // it is some shared resource
    res, err := hq.Get()
    if err != nil {
        t.Errorf("wrong Get() response")
    }
    expErr := false
    for i := 0; i < retries; i++ {
        expErr = false
        res.Lock()
        sharedCon, err = res.TryOpen()
        if err == nil {
            con = sharedCon.(*Connection)
            err, expErr = con.Ping(), true
            if err == nil {
                break
            }
        }
        res.Unlock()
        res.Clean(-1)
        // wait reconnect...
        loggerDebug.Printf("wait reconnect [%v] %v", i, &res)
    }
    loggerDebug.Printf("run start for %v - %v\n", &res, err)
    defer func() {
        loggerDebug.Printf("run end for %v\n", &res)
        if err == nil {
            res.Unlock()
        }
    }()
    if err != nil {
        result <- Result{0, 0, err, expErr}
        return
    }
    if !con.Active {
        t.Errorf("inactive connection was used")
    }
    salt := time.Duration(rand.Int63n(maxReqTime)) * time.Millisecond
    time.Sleep(minReqTime + salt)
    // loggerDebug.Printf("send to channel: %v-%v", req.ID, con.ID)
    result <- Result{req.ID, req.ID * con.ID, nil, false}
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
    hq.Clean(false)
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
            if result.ExpErr {
                t.Logf("expexted error %v", result)
            } else {
                if result.Err != nil {
                    t.Errorf("result error: %v", result.Err)
                }
                if result.Answer == 0 {
                    t.Errorf("incorrect answer: %v", result)
                }
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
