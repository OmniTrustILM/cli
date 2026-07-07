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

import "github.com/spf13/cobra"

// GroupID identifies a cobra help-group. The infra/resources/diagnostics taxonomy
// lives in --help grouping, NOT in required typed path segments.
type GroupID string

// Standard help-group IDs. Commands set GroupID in their GroupAnnotationKey annotation.
const (
	GroupInfrastructure GroupID = "infrastructure"
	GroupResources      GroupID = "resources"
	GroupDiagnostics    GroupID = "diagnostics"
	GroupOther          GroupID = "other"
)

// registerGroups attaches the standard help-groups to the root command.
func registerGroups(root *cobra.Command) {
	root.AddGroup(
		&cobra.Group{ID: string(GroupInfrastructure), Title: "Infrastructure Commands:"},
		&cobra.Group{ID: string(GroupResources), Title: "Resource Commands:"},
		&cobra.Group{ID: string(GroupDiagnostics), Title: "Diagnostics Commands:"},
		&cobra.Group{ID: string(GroupOther), Title: "Other Commands:"},
	)
}
