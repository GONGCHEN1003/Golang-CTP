// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goctp

//#cgo LDFLAGS: -fPIC -L. -L/home/shaoguang/go/src/github.com/qerio/goctp/lib -Wl,-rpath=/home/shaoguang/go/src/github.com/qerio/goctp/lib -lthostmduserapi -lthosttraderapi -lstdc++
//#cgo CPPFLAGS: -fPIC -I. -I/home/shaoguang/go/src/github.com/qerio/goctp/api/ThostTraderApi_v6.3.6_20160606_linux64
import "C"

