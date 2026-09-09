/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
