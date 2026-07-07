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

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/version"
)

// newVersionCommand prints the client + build info. When a cluster is reachable,
// operator/platform versions observed from the cluster are added to the output.
func newVersionCommand(o *Options) *cobra.Command {
	var (
		short  bool
		output string
	)
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print the ilmctl client version and build info",
		GroupID: string(GroupOther),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := version.Client()
			if o != nil && o.Factory != nil {
				fillClusterVersions(cmd.Context(), o.Factory, &info)
			}
			return printVersionInfo(cmd.OutOrStdout(), info, short, output)
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "Print just the client version string.")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: json|yaml.")
	return cmd
}

// printVersionInfo writes version info to w in the requested format.
func printVersionInfo(w io.Writer, info version.Info, short bool, output string) error {
	if short {
		_, err := fmt.Fprintln(w, info.ClientVersion)
		return err
	}
	switch output {
	case "json":
		return printVersionJSON(w, info)
	case "yaml":
		return printVersionYAML(w, info)
	case "":
		return printVersionHuman(w, info)
	default:
		return UsageError{fmt.Errorf("unsupported output format %q (use json|yaml)", output)}
	}
}

func printVersionJSON(w io.Writer, info version.Info) error {
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func printVersionYAML(w io.Writer, info version.Info) error {
	b, err := yaml.Marshal(info)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, string(b))
	return err
}

func printVersionHuman(w io.Writer, info version.Info) error {
	lines := []string{
		fmt.Sprintf("Client Version: %s", info.ClientVersion),
		fmt.Sprintf("Git Commit:     %s", info.GitCommit),
		fmt.Sprintf("Build Date:     %s", info.BuildDate),
		fmt.Sprintf("Go Version:     %s", info.GoVersion),
		fmt.Sprintf("Platform:       %s", info.Platform),
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// fillClusterVersions populates operator/platform versions from the cluster
// (best-effort). A missing or unreachable cluster does NOT fail version; the
// command still prints the client-side info.
func fillClusterVersions(ctx context.Context, f *k8s.Factory, info *version.Info) {
	c, err := f.Client()
	if err != nil {
		return
	}
	list, err := c.ListPlatforms(ctx, "")
	if err != nil {
		return
	}
	applyPlatformVersions(list, info)
}

// applyPlatformVersions populates info from a PlatformList: the first platform
// with a non-empty ObservedVersion sets OperatorVersion; every non-empty
// ObservedVersion is appended to PlatformVersions.
func applyPlatformVersions(list *otilmv1alpha1.PlatformList, info *version.Info) {
	for i := range list.Items {
		v := list.Items[i].Status.ObservedVersion
		if v == "" {
			continue
		}
		info.PlatformVersions = append(info.PlatformVersions, v)
		if info.OperatorVersion == "" {
			info.OperatorVersion = v
		}
	}
}
