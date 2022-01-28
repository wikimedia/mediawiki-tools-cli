/*Package cmd is used for command line.

Copyright © 2021 Addshore

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/spf13/cobra"
)

func NewDebugCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "debug",
		Hidden: true,
	}
}

func debugAttachToCmd(rootCmd *cobra.Command) {
	debugCmd := NewDebugCmd()
	rootCmd.AddCommand(debugCmd)
	debugEventsCmd := NewDebugEventsCmd()
	debugCmd.AddCommand(NewDebugEventsCmd())
	debugEventsCmd.AddCommand(NewDebugEventsEmitCmd())
}
