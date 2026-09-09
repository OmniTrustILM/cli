/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package generate

// genExternal and genManaged carry their own literals rather than aliasing the
// production constants modeExternal/modeManaged. They are the EXPECTED values of
// the profile-seed assertions, and the seeds are built from those production
// constants — aliasing would make the assertions tautological and stop them
// pinning the CRD wire values.
const (
	genAPIKey            = "apiKey"
	genCryptosense       = "cryptosense"
	genILMURL            = "https://ilm.example.com"
	genExternal          = "external"
	genGatewayAPI        = "gatewayAPI"
	genLetsEncrypt       = "letsEncrypt"
	genManaged           = "managed"
	genEgress            = "egress"
	genEgressConfigToken = "egress-config-token"
	genConnectorDemo     = "connector-demo"
	genTLSSource         = "tls-source"
	genEffectiveValues   = "# Effective values"
	genILM               = "ilm" // the name/namespace the generator tests scaffold into
	genMyApp             = "myapp"
	genEmptyName         = "empty name"
)
