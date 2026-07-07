/*
Copyright (c) ILM.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package cli

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

// init silences the Kubernetes client libraries' internal logging. client-go and
// apimachinery log transport and discovery-cache retry failures — e.g. the
// discovery cache's repeated `"Unhandled Error" … connection refused` — straight
// to stderr via klog and runtime.ErrorHandlers. For a user-facing CLI that
// surfaces its own errors through cobra and the Printer, that library spam is
// noise that makes an ordinary connection failure look like a crash. Route klog
// to a discard logger and drop the default error handlers so the user sees only
// the CLI's own clean error. This touches only library logging, never the CLI's
// stdout/stderr.
func init() {
	klog.SetLogger(logr.Discard())
	runtime.ErrorHandlers = nil
}
