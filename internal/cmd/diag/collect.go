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

// Package diag implements the diagnostics collect/analyze commands.
package diag

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	opcap "github.com/OmniTrustILM/operator/pkg/capabilities"
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/bundle"
	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/cli"
)

// collectOptions holds the parsed diagnostics-collect flags.
type collectOptions struct {
	NoRedact      bool
	AssumeYes     bool
	IncludeLogs   bool
	Since         time.Duration
	AllNamespaces bool
	Namespace     string
	Format        string
	Output        string
	OutputDir     string
	Sign          bool
}

func newCollectOptions() *collectOptions {
	return &collectOptions{
		IncludeLogs: true,
		Format:      string(bundle.FormatZip),
	}
}

// toCollectOptions validates flags and maps them to bundle.CollectOptions.
// It is a pure function: it accesses no cluster and performs no I/O.
func (o *collectOptions) toCollectOptions() (bundle.CollectOptions, error) {
	if o.NoRedact && !o.AssumeYes {
		return bundle.CollectOptions{}, cli.NewUsageError(fmt.Errorf("--no-redact requires -y/--yes to confirm collecting unredacted secrets"))
	}
	switch bundle.Format(o.Format) {
	case bundle.FormatZip, bundle.FormatTGZ:
		// valid
	default:
		return bundle.CollectOptions{}, cli.NewUsageError(fmt.Errorf("invalid --format %q (want zip|tgz)", o.Format))
	}
	if o.Output != "" && o.OutputDir != "" {
		return bundle.CollectOptions{}, cli.NewUsageError(fmt.Errorf("--output and --output-dir are mutually exclusive"))
	}
	co := bundle.CollectOptions{
		AllNamespaces: o.AllNamespaces,
		IncludeLogs:   o.IncludeLogs,
		Since:         o.Since,
		Redact:        !o.NoRedact,
		Format:        bundle.Format(o.Format),
		Sign:          o.Sign,
	}
	if !o.AllNamespaces && o.Namespace != "" {
		co.Namespaces = []string{o.Namespace}
	}
	return co, nil
}

// NewDiagnosticsCommand builds the `diagnostics` command (default action = collect)
// and registers the `analyze` subcommand.
func NewDiagnosticsCommand(o *cli.Options) *cobra.Command {
	co := newCollectOptions()
	cmd := &cobra.Command{
		Use:     "diagnostics",
		Aliases: []string{"diag"},
		GroupID: string(cli.GroupDiagnostics),
		Short:   "Collect a redacted, versioned ILM support bundle",
		Long: "Collect a portable support bundle: versions, configuration (all CR specs+status, " +
			"non-secret), state (conditions, phases, events, managed-infra, capability report, " +
			"cluster/node/CRD info) and component logs. Secrets are redacted by default.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCollect(cmd.Context(), o, co)
		},
	}
	fs := cmd.Flags()
	fs.BoolVar(&co.NoRedact, "no-redact", false, "collect unredacted secrets (requires -y)")
	fs.BoolVarP(&co.AssumeYes, "yes", "y", false, "assume yes to confirmations")
	fs.BoolVar(&co.IncludeLogs, "include-logs", true, "include component logs")
	fs.DurationVar(&co.Since, "since", 0, "only logs newer than this duration")
	fs.BoolVarP(&co.AllNamespaces, "all-namespaces", "A", false, "collect across all namespaces")
	fs.StringVarP(&co.Namespace, "namespace", "n", "", "namespace to collect")
	fs.StringVar(&co.Format, "format", string(bundle.FormatZip), "archive format: zip|tgz")
	fs.StringVar(&co.Output, "output", "", "write the bundle to this file")
	fs.StringVar(&co.OutputDir, "output-dir", "", "unpack the bundle into this directory (CI-friendly)")
	fs.BoolVar(&co.Sign, "sign", false, "sign the bundle with cosign")

	cmd.AddCommand(newAnalyzeCommand(o))
	return cmd
}

func runCollect(ctx context.Context, o *cli.Options, fo *collectOptions) error {
	opts, err := fo.toCollectOptions()
	if err != nil {
		return err
	}
	client, err := o.Factory.Client()
	if err != nil {
		return err
	}
	if err := resolveCollectNamespace(o, &opts); err != nil {
		return err
	}

	var reporter *capabilities.Reporter
	if client.Mapper != nil {
		reporter = capabilities.NewReporter(opcap.New(client.Mapper))
	}
	collector := bundle.NewCollector(client, reporter)

	var buf bytes.Buffer
	m, err := collector.Collect(ctx, opts, &buf)
	if err != nil {
		return err
	}

	if opts.Format == bundle.FormatZip && fo.OutputDir != "" {
		return writeUnpacked(o, buf.Bytes(), fo.OutputDir, m)
	}
	return writeArchiveFile(ctx, o, fo, &buf, opts, m)
}

// resolveCollectNamespace fills opts.Namespaces from the kubeconfig context when
// neither --all-namespaces nor an explicit namespace was provided.
func resolveCollectNamespace(o *cli.Options, opts *bundle.CollectOptions) error {
	if opts.AllNamespaces || len(opts.Namespaces) > 0 {
		return nil
	}
	ns, _, err := o.Factory.Namespace()
	if err != nil {
		return err
	}
	opts.Namespaces = []string{ns}
	return nil
}

// writeUnpacked extracts a zip bundle into dir and prints a summary line.
func writeUnpacked(o *cli.Options, raw []byte, dir string, m bundle.Manifest) error {
	if err := unpackZip(raw, dir); err != nil {
		return err
	}
	_, err := fmt.Fprintf(o.Out, "Bundle unpacked to %s (%d files, %d skipped)\n", dir, len(m.Files), len(m.Skipped))
	return err
}

// writeArchiveFile persists the bundle buffer to a file, then optionally signs it.
func writeArchiveFile(ctx context.Context, o *cli.Options, fo *collectOptions, buf *bytes.Buffer, opts bundle.CollectOptions, m bundle.Manifest) error {
	dest := fo.Output
	if dest == "" {
		dest = defaultBundleName(fo.Format)
	}
	f, err := os.Create(dest) //nolint:gosec // dest is a user-supplied path; intentional.
	if err != nil {
		return err
	}
	if _, err := writeArchive(f, buf); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(o.Out, "Bundle written to %s (%d files, %d skipped)\n", dest, len(m.Files), len(m.Skipped)); err != nil {
		return err
	}
	if !opts.Sign {
		return nil
	}
	sig, serr := bundle.Sign(ctx, dest)
	if serr != nil {
		return serr
	}
	_, err = fmt.Fprintf(o.Out, "Signature written to %s\n", sig)
	return err
}

// defaultBundleName returns a timestamped filename for the bundle archive.
func defaultBundleName(format string) string {
	ext := ".zip"
	if format == string(bundle.FormatTGZ) {
		ext = ".tgz"
	}
	return fmt.Sprintf("ilm-diagnostics-%s%s", time.Now().UTC().Format("20060102-150405"), ext)
}

func writeArchive(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

func unpackZip(raw []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		//nolint:gosec // filepath.Clean prevents zip-slip path traversal.
		target := filepath.Join(dir, filepath.Clean("/"+f.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		body, rerr := io.ReadAll(rc)
		_ = rc.Close()
		if rerr != nil {
			return rerr
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			return err
		}
	}
	return nil
}
