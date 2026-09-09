/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package connector

import "k8s.io/apimachinery/pkg/runtime"

// runtimeObject lets the per-package fake-client helper accept typed CRs.
type runtimeObject = runtime.Object

// testNS is the namespace used across hermetic connector tests.
const testNS = "ns1"

// fmtJSON is the -o flag value for JSON structured output.
const fmtJSON = "json"

const (
	connectorFlagImage = "--image"
	connectorFlagName  = "--name"
	connectorAvailable = "Available"
	connectorFlagNS    = "--namespace"
	connectorDemoName  = "demo"
	connectorNamespace = "ilm"
)
