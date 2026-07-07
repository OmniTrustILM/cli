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

package connector

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/generate"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

// connectorClientFor is the injectable seam that resolves a *k8s.Client.
// Tests override it to inject a fake without a live apiserver.
var connectorClientFor = func(o *cli.Options) (*k8s.Client, error) { return o.Factory.Client() }

// NewGenerateCommand builds `connector generate`.
func NewGenerateCommand(o *cli.Options) *cobra.Command {
	var (
		name, namespace, image string
		platformURL, regName   string
		authType               string
		replicas               int32
		apply                  bool
		dryRun                 string
		force                  bool
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Scaffold a Connector CR",
		Long: "Scaffold a Connector custom resource. Pass --platform-url and --name to add a " +
			"registration block; note that connector registration needs a running platform " +
			"(the operator registers the connector against the live platform URL), so apply the " +
			"registration only once the platform is up. Use --apply to server-side apply directly.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			set := shared.ChangedFlags(cmd)
			var replicasPtr *int32
			if set["replicas"] {
				v := replicas
				replicasPtr = &v
			}
			c, notes, err := generate.ScaffoldConnector(generate.ConnectorOptions{
				Name:        name,
				Namespace:   namespace,
				Image:       image,
				PlatformURL: platformURL,
				RegName:     regName,
				AuthType:    authType,
				Replicas:    replicasPtr,
			})
			if err != nil {
				return err
			}
			if apply {
				client, cerr := connectorClientFor(o)
				if cerr != nil {
					return cerr
				}
				return shared.ApplyObject(cmd.Context(), o, client, c, dryRun, force)
			}
			yaml, err := generate.Render(c, notes)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(o.Out, yaml)
			return nil
		},
	}

	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if platformURL != "" && regName == "" {
			regName = name
		}
		return nil
	}

	fs := cmd.Flags()
	fs.StringVar(&name, "name", "", "Connector resource name (required)")
	fs.StringVarP(&namespace, "namespace", "n", "", "Target namespace")
	fs.StringVar(&image, "image", "", "Container image (repository/name:tag, required)")
	fs.StringVar(&platformURL, "platform-url", "", "Platform URL to register the connector with")
	fs.StringVar(&regName, "name-registration", "", "Registration name (defaults to --name when registering)")
	fs.StringVar(&authType, "auth-type", "none", "Registration auth type: none|basic|certificate|apiKey|jwt")
	fs.Int32Var(&replicas, "replicas", 1, "Number of replicas")
	fs.BoolVar(&apply, "apply", false, "Server-side apply the generated CR (field manager ilmctl)")
	fs.StringVar(&dryRun, "dry-run", "none", "Dry-run mode for --apply: none|client|server")
	fs.BoolVar(&force, "force-conflicts", false, "Force-apply on field-manager conflicts")

	return cmd
}
