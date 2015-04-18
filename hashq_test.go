// Copyright (c) 2015, Alexander Zaytsev. All rights reserved.
// Use of this source code is governed by a LGPL-style
// license that can be found in the LICENSE file.

package hashq

import (
    "testing"
)

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
