/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import "k8s.io/apimachinery/pkg/runtime"

// runtimeObject lets the per-package fake-client helper accept typed CRs.
type runtimeObject = runtime.Object

// testNS is the namespace used across hermetic platform tests.
const testNS = "ns1"

// fmtJSON is the -o flag value for JSON structured output.
const fmtJSON = "json"

const (
	platformVer2170   = "2.17.0"
	platformVer2180   = "2.18.0"
	platformAdminCert = "admin-cert"
	platformAdminPwd  = "admin-password"
	platformAPIGroup  = "otilm.com/v1alpha1"
	platformKind      = "Platform"
	platformPassword  = "password"
	platformAvailable = "Available"
	platformRunning   = "Running"
	platformFlagName  = "--name"
	platformFlagProf  = "--profile"
	platformExternal  = "external"
	platformName      = "ilm" // the fixture platform's name; the tests deploy it into a namesake namespace
	platformFlagNS    = "--namespace"
	platformFlagTo    = "--to"
	platformILMDomain = "ilm.example.com"
	platformDeleted   = "deleted"
	errNoDowngrade    = "no downgrade"
)
