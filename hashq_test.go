// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a LGPL-style
// license that can be found in the LICENSE file.

package hashq

import (
    // "fmt"
    "math/rand"
    "sync"
    "testing"
    "time"
)

const (
    poolSize = 16
    maxVal   = 10
)

var (
    maxRequests          = 1024
    cleanPeriod          = 8 * time.Millisecond
    maxTaskDelay   int64 = 32 // time.Millisecond
    delayCreation  int64 = 4  // time.Millisecond
    waitAfterClose       = 1 * time.Microsecond
)

type Conn struct {
    ID    int
    mutex sync.RWMutex
    one   sync.Once
}

func (c *Conn) New() Shared {
    return &Conn{}
}

func (c *Conn) Close(d time.Duration) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    // it is only example
    if c.ID != 0 {
        c.ID = 0
        c.one = sync.Once{}
    }
    time.Sleep(d)
}

func (c *Conn) CanClose() bool {
    return c.ID != 0
}

func (c *Conn) Open(v int) {
    c.mutex.RLock()
    // loggerDebug.Printf("locked %p", c)
    f := func() {
        c.ID += v
    }
    c.one.Do(f)
}

func (c *Conn) Release() {
    c.mutex.RUnlock()
    // loggerDebug.Printf("unlocked %p", c)
}

func GetConn(v int, ch <-chan Shared) *Conn {
    shared := <-ch
    conn := shared.(*Conn)
    conn.Open(v)
    return conn
}

func delay(d int64) {
    rand.Seed(time.Now().UnixNano())
    time.Sleep(time.Duration(rand.Int63n(d)) * time.Millisecond)
}

func Task(ch chan Shared, result chan *Conn) {
    rand.Seed(time.Now().UnixNano())
    delay(maxTaskDelay)
    conn := GetConn(rand.Intn(maxVal-1)+1, ch)
    // defer conn.Release()
    delay(maxTaskDelay)
    result <- conn
}

func TestNew(t *testing.T) {
    Debug(true)
    e := &Conn{}
    pool := New(-1, e, waitAfterClose)
    if pool == nil {
        t.Errorf("incorrect behavior")
        return
    }
    ch, ec := make(chan Shared, 4), make(chan error)
    go pool.Produce(ch, ec)
    if err := <-ec; err == nil {
        t.Errorf("incorrect behavior")
        return
    }
    pool = New(poolSize, e, waitAfterClose)
    go pool.Produce(ch, ec)
    if err := <-ec; err != nil {
        t.Errorf("invalid state: %v", err)
        return
    }
    go pool.Monitor(cleanPeriod)
    stop, result := make(chan int), make(chan *Conn, 2)
    go func() {
        j := 0
        for c := range result {
            // loggerDebug.Printf("con=%p, id=%v", c, c.ID)
            switch {
            case c.ID < 1:
                t.Errorf("wrong value: %v %v", c.ID, c)
            case c.ID > maxVal:
                t.Errorf("wrong value: %v %v", c.ID, c)
            }
            c.Release()
            j++
            if j >= maxRequests {
                close(result)
            }
            if j%50 == 0 {
                loggerDebug.Printf("%v task competed", j)
            }
        }
        stop <- j
    }()
    for i := 0; i < maxRequests; i++ {
        delay(delayCreation)
        go Task(ch, result)
    }
    t.Logf("all %v tasks finished", <-stop)
    defer time.Sleep(cleanPeriod)
}
