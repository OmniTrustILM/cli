//go:build e2e

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

// Package e2e drives the built ilmctl binary against a real Kubernetes cluster
// reachable via the ambient KUBECONFIG. It is compiled only under the `e2e`
// build tag (so `go test ./...` and `make verify` never touch it) and is meant
// to be run by `make e2e`, which provisions a Kind cluster, builds the binary,
// and exports ILM_BIN + KUBECONFIG.
//
// The suite skips cleanly (never fails) when its preconditions are absent:
//   - no reachable cluster, or
//   - the operator source checkout (ILM_OPERATOR_DIR, default ../../../operator)
//     is missing.
//
// The operator install uses `--from-source <checkout>`; that is a development
// test harness convenience, not a product coupling.
package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	operatorNamespace = "ilm-operator-system"
	keycloakNamespace = "keycloak"
	platformNamespace = "ilm"

	// envAllNamespaces is the value the keycloak-operator env vars must carry so
	// the operator watches every namespace rather than only its own.
	envAllNamespaces = "JOSDK_ALL_NAMESPACES"
)

// ilmBin holds the path to the ilmctl binary under test, resolved in TestMain.
var ilmBin string

// TestMain resolves the ilmctl binary once for the whole suite: it prefers the
// ILM_BIN env var set by the Makefile, and otherwise `go build`s a throwaway
// binary so the suite is runnable with a bare `go test -tags e2e`.
func TestMain(m *testing.M) {
	bin, cleanup, err := resolveBinary()
	if err != nil {
		// Cannot build the binary: report and skip the whole run by exiting 0
		// after printing a diagnostic. A build failure here is an environment
		// problem, not a test failure.
		os.Stderr.WriteString("e2e: cannot resolve ilmctl binary: " + err.Error() + "\n")
		os.Exit(1)
	}
	ilmBin = bin
	code := m.Run()
	if cleanup != nil {
		cleanup()
	}
	os.Exit(code)
}

// resolveBinary returns the ilmctl binary path. If ILM_BIN is set and points at
// an existing file it is used directly; otherwise the binary is built into a
// temp dir and a cleanup func removes it.
func resolveBinary() (bin string, cleanup func(), err error) {
	if b := os.Getenv("ILM_BIN"); b != "" {
		abs, aerr := filepath.Abs(b)
		if aerr != nil {
			return "", nil, aerr
		}
		if _, serr := os.Stat(abs); serr != nil {
			return "", nil, serr
		}
		return abs, nil, nil
	}

	dir, err := os.MkdirTemp("", "ilmctl-e2e-bin")
	if err != nil {
		return "", nil, err
	}
	out := filepath.Join(dir, "ilmctl")
	// The test file lives at <repo>/test/e2e; the module root is two levels up.
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command("go", "build", "-o", out, "./cmd/ilmctl")
	cmd.Dir = repoRoot
	if combined, berr := cmd.CombinedOutput(); berr != nil {
		return "", nil, errors.New("go build failed: " + berr.Error() + "\n" + string(combined))
	}
	return out, func() { _ = os.RemoveAll(dir) }, nil
}

// operatorDir resolves the local operator checkout used for --from-source. It
// honours ILM_OPERATOR_DIR and otherwise defaults to a sibling `operator`
// worktree relative to the repo root, resolved to an absolute path so the CLI's
// path-based source resolution is unambiguous.
func operatorDir(t *testing.T) string {
	t.Helper()
	if d := os.Getenv("ILM_OPERATOR_DIR"); d != "" {
		abs, err := filepath.Abs(d)
		if err != nil {
			t.Fatalf("resolve ILM_OPERATOR_DIR %q: %v", d, err)
		}
		return abs
	}
	// Default: <repo>/../operator (the test binary runs from test/e2e).
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", "operator"))
	if err != nil {
		t.Fatalf("resolve default operator dir: %v", err)
	}
	return abs
}

// run executes the ilmctl binary with args, feeding stdin, and returns combined
// stdout, stderr and the exit code. It never t.Fatals; callers assert.
func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, ilmBin, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("ilmctl %s: exec error: %v", strings.Join(args, " "), err)
		}
	}
	t.Logf("$ ilmctl %s\n[exit %d]\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, outBuf.String(), errBuf.String())
	return outBuf.String(), errBuf.String(), code
}

// kubectl runs kubectl with args against the ambient KUBECONFIG.
func kubectl(t *testing.T, args ...string) (stdout string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("kubectl %s: exec error: %v\n%s", strings.Join(args, " "), err, errBuf.String())
		}
	}
	return outBuf.String(), code
}

// clusterReachable reports whether the ambient KUBECONFIG points at a live
// apiserver. Used to skip the suite cleanly when no cluster is available.
func clusterReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "version", "-o", "json")
	// A reachable server makes `kubectl version` exit 0 with serverVersion in
	// the JSON. We only need the exit code.
	return cmd.Run() == nil
}

// requirePreconditions skips the test (never fails) when a cluster or operator
// source is unavailable, and returns the operator dir when both are present.
func requirePreconditions(t *testing.T) string {
	t.Helper()
	if !clusterReachable() {
		t.Skip("e2e: no reachable Kubernetes cluster via KUBECONFIG; skipping")
	}
	opDir := operatorDir(t)
	crds := filepath.Join(opDir, "deploy", "manifests", "ilm-operator.crds.yaml")
	ctrl := filepath.Join(opDir, "deploy", "manifests", "ilm-operator.yaml")
	if _, err := os.Stat(crds); err != nil {
		t.Skipf("e2e: operator source not found (%s missing); set ILM_OPERATOR_DIR; skipping", crds)
	}
	if _, err := os.Stat(ctrl); err != nil {
		t.Skipf("e2e: operator source not found (%s missing); set ILM_OPERATOR_DIR; skipping", ctrl)
	}
	return opDir
}

// TestSmoke drives the core happy-path flow end-to-end against the cluster.
// Sub-tests run in sequence because each depends on the operator installed by
// the first step.
func TestSmoke(t *testing.T) {
	opDir := requirePreconditions(t)

	t.Run("Init", func(t *testing.T) {
		stdout, stderr, code := run(t, "",
			"init",
			"--from-source", opDir,
			"-n", operatorNamespace,
			"--create-namespace",
			"--wait",
			"--timeout", "5m",
		)
		if code != 0 {
			t.Fatalf("init exited %d; stderr:\n%s", code, stderr)
		}
		// The apply summary is printed to stdout/stderr; the operator Deployment
		// must exist and be Available after --wait returns.
		combined := stdout + stderr
		if !strings.Contains(combined, "Applied") && !strings.Contains(combined, "applied") &&
			!strings.Contains(combined, "unchanged") {
			t.Errorf("init output lacked an apply summary; got:\n%s", combined)
		}
		waitDeploymentAvailable(t, operatorNamespace, 5*time.Minute)
	})

	t.Run("Status", func(t *testing.T) {
		stdout, stderr, code := run(t, "", "status", "-A")
		// status is informational; it must not error out (usage exit 2 or a
		// hard failure). It may exit 0 always for a healthy/empty cluster.
		if code == 2 {
			t.Fatalf("status returned usage error (exit 2); stderr:\n%s", stderr)
		}
		if strings.TrimSpace(stdout+stderr) == "" {
			t.Errorf("status produced no output")
		}
	})

	t.Run("Check", func(t *testing.T) {
		stdout, stderr, code := run(t, "", "check")
		// check runs the analyzer; exit 0 (no fail findings) or 1 (fail
		// findings present) are both documented, sane outcomes. Only a usage
		// error (2) or a crash is a test failure.
		if code == 2 {
			t.Fatalf("check returned usage error (exit 2); stderr:\n%s", stderr)
		}
		if strings.TrimSpace(stdout+stderr) == "" {
			t.Errorf("check produced no output")
		}
	})

	t.Run("PlatformGenerateApplyDryRun", func(t *testing.T) {
		// generate → server-side dry-run apply: validates that a scaffolded
		// Platform is a well-formed, CRD-recognised object that reaches the
		// apiserver's CEL evaluation end to end.
		//
		// The generated scaffold is deliberately a template: an `external`
		// profile sets database.mode=external / messaging.mode=external but
		// leaves the operator-required host/credentials fields for the user to
		// fill. Server-side dry-run therefore returns the operator's CEL
		// business-rule errors ("... is required when mode=external"). That is
		// the SUCCESS signal here: the object passed the CRD structural schema
		// (correct apiVersion/kind, no unknown/mistyped fields) and reached CEL.
		// A structural/schema rejection, by contrast, means generate produced a
		// malformed CR and is a real failure.
		gen, genErr, genCode := run(t, "",
			"platform", "generate",
			"--profile", "external",
			"--name", "ilm",
			"--namespace", platformNamespace,
		)
		if genCode != 0 {
			t.Fatalf("platform generate exited %d; stderr:\n%s", genCode, genErr)
		}
		if !strings.Contains(gen, "kind: Platform") || !strings.Contains(gen, "otilm.com/v1alpha1") {
			t.Fatalf("generate did not produce a Platform CR; got:\n%s", gen)
		}

		// The target namespace must exist for a namespaced server-side apply.
		kubectl(t, "create", "namespace", platformNamespace)

		stdout, stderr, code := run(t, gen,
			"platform", "apply",
			"-f", "-",
			"--dry-run", "server",
		)
		if code == 0 {
			return // fully CEL-valid: strongest possible pass.
		}
		// Non-zero: the only acceptable rejection is the operator's CEL
		// business-rule requirement that the user complete the external-mode
		// fields (message shape: "... required when mode=external"). Any other
		// error (structural schema, unknown field, wrong apiVersion/kind,
		// not-found CRD) is a genuine failure.
		combined := stdout + stderr
		// Guard first against the object never reaching CEL (e.g. CRD absent /
		// schema rejection): those mean generate produced a malformed CR.
		if strings.Contains(combined, "no matches for kind") ||
			strings.Contains(combined, "unknown field") ||
			strings.Contains(combined, "could not find the requested resource") {
			t.Fatalf("platform apply --dry-run=server reached the apiserver but the CR was not CRD-recognised:\n%s", combined)
		}
		// The Platform.otilm.com "... is invalid: [...required when mode=...]"
		// message is the CEL business-rule signal: the object was CRD-valid,
		// admitted to the CRD, and evaluated by the operator's validation rules.
		if !strings.Contains(combined, "Platform.otilm.com") ||
			!strings.Contains(combined, "required when mode=") {
			t.Fatalf("platform apply --dry-run=server exited %d with a non-CEL rejection; the generated Platform is not CRD-valid.\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		t.Logf("generate produced a CRD-valid Platform; server-side CEL correctly flagged the unfilled external-mode fields")
	})

	t.Run("DiagnosticsCollectAnalyze", func(t *testing.T) {
		bundlePath := filepath.Join(t.TempDir(), "bundle.zip")
		stdout, stderr, code := run(t, "",
			"diagnostics",
			"-A",
			"--output", bundlePath,
		)
		if code != 0 {
			t.Fatalf("diagnostics collect exited %d; stderr:\n%s", code, stderr)
		}
		if !strings.Contains(stdout, "Bundle written to") {
			t.Errorf("diagnostics output lacked confirmation; got:\n%s", stdout)
		}
		if fi, err := os.Stat(bundlePath); err != nil || fi.Size() == 0 {
			t.Fatalf("bundle not written or empty at %s: %v", bundlePath, err)
		}
		assertValidZip(t, bundlePath)

		anStdout, anStderr, anCode := run(t, "", "diagnostics", "analyze", bundlePath)
		// analyze exits 0 (no fail finding) or 1 (fail finding). A usage error
		// (2) or a read failure is a test failure.
		if anCode == 2 {
			t.Fatalf("analyze returned usage error (exit 2); stderr:\n%s", anStderr)
		}
		if !strings.Contains(anStdout, "ILM Diagnostics") {
			t.Errorf("analyze did not render the diagnostics report; got:\n%s", anStdout)
		}
	})
}

// TestDepsKeycloak is the key live validation of internal/manifest/deps.go
// against the REAL upstream Keycloak 26.6.3 manifest. It installs the
// keycloak-operator through the CLI, then asserts:
//   - the keycloak-operator Deployment becomes Available,
//   - each container carries the two all-namespaces env vars set to
//     JOSDK_ALL_NAMESPACES (not the upstream default JOSDK_WATCH_CURRENT), and
//   - the synthesized ClusterRole + ClusterRoleBindings exist and reference the
//     upstream cluster roles keycloak{,realmimport}controller-cluster-role.
//
// It fetches manifests from the internet, so it is gated behind ILM_E2E_DEPS=1
// to keep the default run lighter and offline-friendly; `make e2e` sets it.
func TestDepsKeycloak(t *testing.T) {
	if os.Getenv("ILM_E2E_DEPS") != "1" {
		t.Skip("e2e: set ILM_E2E_DEPS=1 to run the Keycloak deps live validation (needs internet)")
	}
	if !clusterReachable() {
		t.Skip("e2e: no reachable Kubernetes cluster via KUBECONFIG; skipping")
	}

	// Install keycloak. The keycloak-operator does not depend on cert-manager,
	// so `--only keycloak` is sufficient.
	stdout, stderr, code := run(t, "", "deps", "install", "--only", "keycloak")
	if code != 0 {
		t.Fatalf("deps install --only keycloak exited %d;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	// The operator Deployment must become Available.
	waitDeploymentAvailable(t, keycloakNamespace, 5*time.Minute)

	// The Deployment name upstream is `keycloak-operator`.
	if _, dcode := kubectl(t, "-n", keycloakNamespace, "get", "deployment", "keycloak-operator"); dcode != 0 {
		t.Fatalf("keycloak-operator Deployment not found in ns %s", keycloakNamespace)
	}

	// Assert the all-namespaces env vars are present with value
	// JOSDK_ALL_NAMESPACES on the operator container. The upstream manifest
	// ships these two keys set to JOSDK_WATCH_CURRENT; the CLI must flip them.
	for _, key := range []string{
		"QUARKUS_OPERATOR_SDK_CONTROLLERS_KEYCLOAKCONTROLLER_NAMESPACES",
		"QUARKUS_OPERATOR_SDK_CONTROLLERS_KEYCLOAKREALMIMPORTCONTROLLER_NAMESPACES",
	} {
		val := envValueFromDeployment(t, key)
		if val != envAllNamespaces {
			t.Errorf("keycloak-operator env %s = %q; want %q (operator would watch only its own namespace)", key, val, envAllNamespaces)
		}
	}

	// Assert the synthesized cluster-scoped RBAC exists.
	assertClusterRoleExists(t, "keycloak-operator-allns-role")
	assertClusterRoleBindingRefs(t, "keycloak-operator-allns-controller", "keycloakcontroller-cluster-role")
	assertClusterRoleBindingRefs(t, "keycloak-operator-allns-realmimport", "keycloakrealmimportcontroller-cluster-role")
	assertClusterRoleBindingRefs(t, "keycloak-operator-allns-role", "keycloak-operator-allns-role")

	// The referenced upstream cluster roles must actually be ClusterRoles (the
	// binding would dangle otherwise).
	assertClusterRoleExists(t, "keycloakcontroller-cluster-role")
	assertClusterRoleExists(t, "keycloakrealmimportcontroller-cluster-role")
}

// waitDeploymentAvailable polls kubectl rollout status for every Deployment in
// ns until each is Available or the timeout elapses.
func waitDeploymentAvailable(t *testing.T, ns string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	names := deploymentNames(t, ns)
	if len(names) == 0 {
		// Retry once briefly: the Deployment object may not be created yet.
		time.Sleep(5 * time.Second)
		names = deploymentNames(t, ns)
	}
	if len(names) == 0 {
		t.Fatalf("no Deployments found in namespace %s", ns)
	}
	for _, name := range names {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("timed out waiting for Deployments in %s to become Available", ns)
		}
		secs := int(remaining.Seconds())
		_, code := kubectl(t, "-n", ns, "rollout", "status", "deployment/"+name,
			"--timeout", (time.Duration(secs) * time.Second).String())
		if code != 0 {
			describe, _ := kubectl(t, "-n", ns, "describe", "deployment/"+name)
			t.Fatalf("Deployment %s/%s did not become Available:\n%s", ns, name, describe)
		}
	}
}

// deploymentNames returns the names of all Deployments in ns.
func deploymentNames(t *testing.T, ns string) []string {
	t.Helper()
	out, code := kubectl(t, "-n", ns, "get", "deployments",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}")
	if code != 0 {
		return nil
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			names = append(names, s)
		}
	}
	return names
}

// envValueFromDeployment reads the value of env var key from the operator
// container of the keycloak-operator Deployment. Returns "" when absent.
func envValueFromDeployment(t *testing.T, key string) string {
	t.Helper()
	out, code := kubectl(t, "-n", keycloakNamespace, "get", "deployment", "keycloak-operator",
		"-o", "jsonpath={range .spec.template.spec.containers[*].env[?(@.name==\""+key+"\")]}{.value}{\"\\n\"}{end}")
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

// assertClusterRoleExists fails the test if the named ClusterRole is absent.
func assertClusterRoleExists(t *testing.T, name string) {
	t.Helper()
	if _, code := kubectl(t, "get", "clusterrole", name); code != 0 {
		t.Errorf("expected ClusterRole %q to exist", name)
	}
}

// assertClusterRoleBindingRefs fails the test if the ClusterRoleBinding is
// absent or does not reference the expected ClusterRole.
func assertClusterRoleBindingRefs(t *testing.T, name, wantRoleRef string) {
	t.Helper()
	out, code := kubectl(t, "get", "clusterrolebinding", name, "-o", "jsonpath={.roleRef.name}")
	if code != 0 {
		t.Errorf("expected ClusterRoleBinding %q to exist", name)
		return
	}
	if got := strings.TrimSpace(out); got != wantRoleRef {
		t.Errorf("ClusterRoleBinding %q roleRef = %q; want %q", name, got, wantRoleRef)
	}
}

// assertValidZip verifies the bundle is a readable, non-empty zip archive.
func assertValidZip(t *testing.T, path string) {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("bundle %s is not a valid zip: %v", path, err)
	}
	defer func() { _ = r.Close() }()
	if len(r.File) == 0 {
		t.Fatalf("bundle %s contains no files", path)
	}
}
