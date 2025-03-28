// Copyright © 2015 Steve Francia <spf@spf13.com>.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"fmt"
	"github.com/inovacc/cobra-cli/internal/project"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"os"
)

func init() {
	addCmd.SetOut(new(bytes.Buffer))
	addCmd.SetErr(new(bytes.Buffer))
}

var (
	packageName string
	parentName  string

	addCmd = &cobra.Command{
		Use:     "add [command name]",
		Aliases: []string{"command"},
		Short:   "Add a command to a Cobra Application",
		Long: `Add (cobra-cli add) will create a new command, with a license and
the appropriate structure for a Cobra-based CLI application,
and register it to its parent (default rootCmd).

If you want your command to be public, pass in the command name
with an initial uppercase letter.

Example: cobra-cli add server -> resulting in a new cmd/server.go`,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var comps []string
			if len(args) == 0 {
				comps = cobra.AppendActiveHelp(comps, "Please specify the name for the new command")
			} else if len(args) == 1 {
				comps = cobra.AppendActiveHelp(comps, "This command does not take any more arguments (but may accept flags)")
			} else {
				comps = cobra.AppendActiveHelp(comps, "ERROR: Too many arguments specified")
			}
			return comps, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				cobra.CheckErr(fmt.Errorf("add needs a name for the command"))
			}

			wd, err := os.Getwd()
			cobra.CheckErr(err)

			commandName := project.ValidateCmdName(args[0])
			newProject := project.NewProject(afero.NewOsFs(), wd, packageName, commandName)
			command := project.NewCommand(commandName, parentName, newProject)

			cobra.CheckErr(command.Create())
			fmt.Printf("%s created at %s\n", command.CmdName, command.AbsolutePath)
		},
	}
)

func init() {
	addCmd.Flags().StringVarP(&packageName, "package", "t", "", "target package name (e.g. github.com/spf13/hugo)")
	addCmd.Flags().StringVarP(&parentName, "parent", "p", "rootCmd", "variable name of parent command for this command")
	cobra.CheckErr(addCmd.Flags().MarkDeprecated("package", "this operation has been removed."))
}
