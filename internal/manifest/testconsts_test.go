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

package manifest

const (
	manifestDeploymentID = "Deployment/ilm-operator-system/mgr"
	manifestOperatorSys  = "ilm-operator-system"
	manifestILMCtl       = "ilmctl"
	manifestPlatformsCRD = "platforms.otilm.com"
	manifestOLMGroup     = "operators.coreos.com"
	manifestCatalogImage = "example.com/idx:1"
	manifestOLMChannel   = "stable"
	manifestOLMV1alpha1  = "v1alpha1"
	manifestTmpYAML      = "/tmp/m.yaml"
	manifestKindFoo      = "kind: Foo\n"

	// Release-asset fixtures shared by the source tests.
	manifestCRDsAsset       = "ilm-operator.crds.yaml"
	manifestControllerAsset = "ilm-operator.yaml"
	manifestChecksumsAsset  = "checksums.txt"
	manifestReleaseTag      = "v1.0.0"
	manifestLatestTag       = "v1.2.3"
	manifestReleasesHost    = "https://github.com/OmniTrustILM/operator/releases"
	manifestCRDsBody        = "kind: CustomResourceDefinition\n"
	manifestControllerBody  = "kind: Deployment\n"
)
