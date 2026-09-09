/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
