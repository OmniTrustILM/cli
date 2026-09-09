/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package generate

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// ProxyOptions are the inputs to ScaffoldProxy.
type ProxyOptions struct {
	Name, Namespace, ConfigTokenSecret, Image string
	Replicas                                  *int32
}

// ScaffoldProxy builds a typed Proxy from a required config-token Secret
// reference and optional image and replicas overrides.
//
// ConfigTokenSecretRef.TokenKey and SigningKeyKey are not set here; the
// operator CRD supplies kubebuilder defaults ("configToken" and
// "tokenSigningKey") that apply at admission time, so omitting them from the
// scaffolded CR is correct and yields a cleaner manifest.
func ScaffoldProxy(o ProxyOptions) (*otilmv1alpha1.Proxy, []EffectiveNote, error) {
	if o.Name == "" {
		return nil, nil, fmt.Errorf("proxy name is required")
	}
	if o.ConfigTokenSecret == "" {
		return nil, nil, fmt.Errorf("--config-token-secret is required")
	}

	notes := []EffectiveNote{
		{Field: "configTokenSecretRef.name", Value: o.ConfigTokenSecret, Source: sourceFlag},
	}

	p := &otilmv1alpha1.Proxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: otilmv1alpha1.GroupVersion.String(),
			Kind:       "Proxy",
		},
		ObjectMeta: metav1.ObjectMeta{Name: o.Name, Namespace: o.Namespace},
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: o.ConfigTokenSecret},
		},
	}

	if o.Image != "" {
		img, err := parseImage(o.Image)
		if err != nil {
			return nil, nil, err
		}
		p.Spec.Image = &otilmv1alpha1.ImageSpec{
			Repository: img.repository,
			Name:       img.name,
			Tag:        img.tag,
			Digest:     img.digest,
		}
		notes = append(notes,
			EffectiveNote{Field: "image.repository", Value: img.repository, Source: sourceFlag},
			EffectiveNote{Field: "image.tag", Value: img.tag, Source: sourceFlag},
		)
		if img.digest != "" {
			notes = append(notes, EffectiveNote{Field: fieldImageDigest, Value: img.digest, Source: sourceFlag})
		}
	}

	if o.Replicas != nil {
		p.Spec.Replicas = o.Replicas
		notes = append(notes, EffectiveNote{Field: "replicas", Value: fmt.Sprintf("%d", *o.Replicas), Source: sourceFlag})
	}

	return p, notes, nil
}
