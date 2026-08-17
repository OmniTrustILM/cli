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

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// ConnectorOptions are the inputs to ScaffoldConnector.
type ConnectorOptions struct {
	Name, Namespace, Image, PlatformURL, RegName, AuthType string
	Replicas                                               *int32
}

// authTypeNone disables registration auth; it is also the generator's default.
const authTypeNone = "none"

var validAuthTypes = map[string]bool{
	authTypeNone: true, "basic": true, "certificate": true, "apiKey": true, "jwt": true,
}

// imageRef holds the parsed components of a container image reference.
type imageRef struct {
	repository string
	name       string
	tag        string
	digest     string
}

// parseImage splits "[registry/path/]name[:tag][@digest]" into imageRef fields.
//
// Digest references use the "@algo:hex" syntax (e.g. "@sha256:..."). When a
// digest is present the name/repository portion is everything before the "@";
// the digest value (e.g. "sha256:abc...") is stored in imageRef.digest.
// A tag may co-exist with a digest ("name:tag@sha256:..."); when present both
// tag and digest are populated. Tag alone works as before.
//
// The Connector CRD's XValidation requires both repository and tag
// (rule="has(self.repository) && has(self.tag)"), so a digest-only reference
// (no tag at all) is rejected here to avoid producing a CRD-invalid manifest.
// To pin by digest, supply the tag that was used to identify the image:
// e.g. "name:1.2@sha256:...".
//
// Registry-port colons (host:5000/repo/name) are not mistaken for tag
// separators because a slash follows the colon in the host:port segment.
func parseImage(ref string) (imageRef, error) {
	if ref == "" {
		return imageRef{}, fmt.Errorf("image is required")
	}

	namePart := ref
	digest := ""

	// Split off the digest component at "@".
	if atIdx := strings.Index(ref, "@"); atIdx >= 0 {
		namePart = ref[:atIdx]
		digest = ref[atIdx+1:] // e.g. "sha256:abc..."
		if digest == "" {
			return imageRef{}, fmt.Errorf("image %q has an empty digest after '@'", ref)
		}
	}

	// Parse the tag from the name portion.
	// The last ":" that is not inside a registry port separates the tag.
	// A ":" inside a registry port is always followed by a path segment
	// containing a slash (host:5000/repo/name), so we require that there is
	// no slash after the candidate tag colon.
	tag := ""
	colonIdx := strings.LastIndex(namePart, ":")
	if colonIdx >= 0 && colonIdx < len(namePart)-1 {
		// Ensure the colon is not part of a registry port: no slash must appear after it.
		if strings.LastIndex(namePart, "/") < colonIdx {
			tag = namePart[colonIdx+1:]
			namePart = namePart[:colonIdx]
		}
	}

	// A digest-only reference cannot satisfy the Connector CRD XValidation
	// (rule="has(self.repository) && has(self.tag)"), so reject it early.
	if digest != "" && tag == "" {
		return imageRef{}, fmt.Errorf(
			"image %q has a digest but no tag; the Connector CRD requires both repository and tag — "+
				"add the tag, e.g. %q",
			ref, namePart+":latest@"+digest,
		)
	}

	// Without a digest we still require a tag.
	if tag == "" {
		return imageRef{}, fmt.Errorf("image %q must include a tag (name:tag) or a digest (name:tag@algo:hex)", ref)
	}

	// Split the remaining name portion into repository prefix and image name.
	if lastSlash := strings.LastIndex(namePart, "/"); lastSlash >= 0 {
		return imageRef{
			repository: namePart[:lastSlash],
			name:       namePart[lastSlash+1:],
			tag:        tag,
			digest:     digest,
		}, nil
	}
	return imageRef{repository: namePart, tag: tag, digest: digest}, nil
}

// ScaffoldConnector builds a typed Connector from the supplied options.
//
// Registration is emitted only when both PlatformURL and RegName are provided;
// supplying one without the other returns a validation error. AuthType is
// validated against the operator's enum (none|basic|certificate|apiKey|jwt)
// regardless of whether registration is included.
func ScaffoldConnector(o ConnectorOptions) (*otilmv1alpha1.Connector, []EffectiveNote, error) {
	if o.Name == "" {
		return nil, nil, fmt.Errorf("connector name is required")
	}
	img, err := parseImage(o.Image)
	if err != nil {
		return nil, nil, err
	}
	authType := o.AuthType
	if authType == "" {
		authType = authTypeNone
	}
	if !validAuthTypes[authType] {
		return nil, nil, fmt.Errorf("invalid auth-type %q (want none|basic|certificate|apiKey|jwt)", authType)
	}

	notes := []EffectiveNote{
		{Field: "image.repository", Value: img.repository, Source: sourceFlag},
		{Field: "image.tag", Value: img.tag, Source: sourceFlag},
	}
	if img.digest != "" {
		notes = append(notes, EffectiveNote{Field: fieldImageDigest, Value: img.digest, Source: sourceFlag})
	}

	c := &otilmv1alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: otilmv1alpha1.GroupVersion.String(),
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{Name: o.Name, Namespace: o.Namespace},
		Spec: otilmv1alpha1.ConnectorSpec{
			Image: otilmv1alpha1.ImageSpec{
				Repository: img.repository,
				Name:       img.name,
				Tag:        img.tag,
				Digest:     img.digest,
			},
		},
	}

	if o.Replicas != nil {
		c.Spec.Replicas = o.Replicas
		notes = append(notes, EffectiveNote{Field: "replicas", Value: fmt.Sprintf("%d", *o.Replicas), Source: sourceFlag})
	}

	switch {
	case o.PlatformURL != "" && o.RegName != "":
		c.Spec.Registration = &otilmv1alpha1.RegistrationSpec{
			PlatformURL: o.PlatformURL,
			Name:        o.RegName,
			AuthType:    otilmv1alpha1.AuthType(authType),
		}
		notes = append(notes,
			EffectiveNote{Field: "registration.authType", Value: authType, Source: sourceFlag},
			EffectiveNote{Field: "registration.name", Value: o.RegName, Source: sourceFlag},
			EffectiveNote{Field: "registration.platformUrl", Value: o.PlatformURL, Source: sourceFlag},
		)
	case o.PlatformURL == "" && o.RegName == "":
		// No registration requested; leave Registration nil.
	default:
		return nil, nil, fmt.Errorf("registration requires both --platform-url and --registration-name")
	}

	return c, notes, nil
}
