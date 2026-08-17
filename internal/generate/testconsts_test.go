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
