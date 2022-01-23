/*Package cmd is used for command line.

Copyright © 2020 Addshore

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
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/eventlogging"
)

var debugEventsCmd = &cobra.Command{
	Hidden: true,
	Use:    "events",
}

var debugEventsEmitCmd = &cobra.Command{
	Hidden: true,
	Use:    "emit",
	Short:  "Emit events now",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Emitting events")
		eventlogging.EmitEvents()
	},
}

func init() {
	debugCmd.AddCommand(debugEventsCmd)
	debugEventsCmd.AddCommand(debugEventsEmitCmd)
}
