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
