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

package platform

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func newPlatformClient(t *testing.T, objs ...interface {
	metav1.Object
	runtimeObject
}) *k8s.Client {
	t.Helper()
	s, err := k8s.NewScheme()
	require.NoError(t, err)
	b := ctrlfake.NewClientBuilder().WithScheme(s)
	for _, o := range objs {
		b = b.WithObjects(o)
	}
	return &k8s.Client{Typed: b.Build(), Scheme: s}
}

func newPlatform(name, ns string) *otilmv1alpha1.Platform {
	return &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status: otilmv1alpha1.PlatformStatus{
			Phase: otilmv1alpha1.PlatformPhaseRunning, ObservedVersion: platformVer2180,
			Conditions: []metav1.Condition{{Type: platformAvailable, Status: metav1.ConditionTrue}},
		},
	}
}

func TestRunGet_ListWide(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	s := out.String()
	assert.Contains(t, s, platformName)
	assert.Contains(t, s, platformRunning)
	assert.Contains(t, s, platformVer2180)
}

func TestRunGet_SingleNotFound(t *testing.T) {
	c := newPlatformClient(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	assert.Error(t, runGet(context.Background(), c, p, testNS, "absent"))
}

func TestRunStatus_ShowsPhaseAndState(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, platformName))
	s := out.String()
	assert.Contains(t, s, platformRunning)
	assert.Contains(t, s, platformName)
}

func TestRunGet_JSONStructured(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runGet(context.Background(), c, p, testNS, platformName))
	s := out.String()
	assert.Contains(t, s, platformName)
}

// TestNewGetCommandFromOpts_RunE exercises the RunE closure via clientFn injection
// so the NewGetCommand constructor and its RunE body reach covered lines.
func TestNewGetCommandFromOpts_RunE_ListWide(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newGetCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{}))
	assert.Contains(t, out.String(), platformName)
}

// TestNewStatusCommandFromOpts_RunE exercises the status RunE via ClientFn injection.
func TestNewStatusCommandFromOpts_RunE(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newStatusCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{platformName}))
	assert.Contains(t, out.String(), platformRunning)
}

// TestNewDescribeCommandFromOpts_RunE exercises the describe RunE via ClientFn injection.
func TestNewDescribeCommandFromOpts_RunE(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: testNS}}
	plat.Spec.Edge = &otilmv1alpha1.EdgeSpec{Host: "ilm.example.com"}
	plat.Status.Phase = otilmv1alpha1.PlatformPhaseRunning
	c := newPlatformClient(t, plat)
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newDescribeCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{platformName}))
	assert.Contains(t, out.String(), "ilm.example.com")
}

// TestNewEventsCommandFromOpts_RunE exercises the events RunE via ClientFn injection.
func TestNewEventsCommandFromOpts_RunE(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newEventsCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{platformName}))
	// no events seeded → "no events" line
	assert.Contains(t, out.String(), "no events")
}

// TestNewPlatformCommand_SubcommandRegistration verifies the parent command registers
// all expected subcommands.
func TestNewPlatformCommand_SubcommandRegistration(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, errOut)}
	cmd := NewPlatformCommand(o)
	assert.Equal(t, "platform", cmd.Use)
	assert.Equal(t, string(cli.GroupResources), cmd.GroupID)
	assert.Contains(t, cmd.Aliases, "plat")
	assert.Contains(t, cmd.Aliases, "platforms")

	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"get", "status", "describe", "events", "wait", "logs"} {
		assert.True(t, names[want], "expected subcommand %q", want)
	}
}

// TestAgeFunction covers all branches of the age helper via cmdutil.
func TestAgeFunction(t *testing.T) {
	assert.Equal(t, "<unknown>", cmdutil.Age(metav1.Time{}.Time))
	// seconds branch
	s := cmdutil.Age(time.Now().Add(-10 * time.Second))
	assert.Contains(t, s, "s")
	// minutes branch
	m := cmdutil.Age(time.Now().Add(-5 * time.Minute))
	assert.Contains(t, m, "m")
	// hours branch
	h := cmdutil.Age(time.Now().Add(-3 * time.Hour))
	assert.Contains(t, h, "h")
	// days branch
	d := cmdutil.Age(time.Now().Add(-48 * time.Hour))
	assert.Contains(t, d, "d")
}

// TestOrNone covers the orNone helper via cmdutil.
func TestOrNone(t *testing.T) {
	assert.Equal(t, "<none>", cmdutil.OrNone(""))
	assert.Equal(t, "foo", cmdutil.OrNone("foo"))
}

// TestRunGet_List_Multiple covers listing multiple platforms.
func TestRunGet_List_Multiple(t *testing.T) {
	c := newPlatformClient(t, newPlatform("ilm1", testNS), newPlatform("ilm2", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	s := out.String()
	assert.Contains(t, s, "ilm1")
	assert.Contains(t, s, "ilm2")
}

// TestRunGet_JSONList covers the structured list path (-o json, no name arg).
func TestRunGet_JSONList(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	assert.Contains(t, out.String(), "PlatformList")
}

// TestRunStatus_WithConditions covers the conditions table path.
func TestRunStatus_WithConditions(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: testNS},
		Status: otilmv1alpha1.PlatformStatus{
			Phase:           otilmv1alpha1.PlatformPhaseRunning,
			ObservedVersion: platformVer2180,
			Conditions: []metav1.Condition{
				{Type: platformAvailable, Status: metav1.ConditionTrue, Reason: "AllReady", Message: "ok"},
			},
		},
	}
	c := newPlatformClient(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, platformName))
	s := out.String()
	assert.Contains(t, s, platformAvailable)
	assert.Contains(t, s, "AllReady")
	assert.Contains(t, s, "ok")
}

// TestRunStatus_JSONStructured covers the -o json path of status.
func TestRunStatus_JSONStructured(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runStatus(context.Background(), c, p, testNS, platformName))
	assert.Contains(t, out.String(), platformName)
}
