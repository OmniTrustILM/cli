/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
