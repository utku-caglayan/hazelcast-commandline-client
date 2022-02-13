/*
 * Copyright (c) 2008-2021, Hazelcast, Inc. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package use

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hazelcast/hazelcast-commandline-client/commands/common"
)

func New() *cobra.Command {
	cmd := cobra.Command{
		Use:   "use {map | queue | multimap} {defaultName | --reset}",
		Short: "sets default name for data structures such as map, queue...",
		Example: "use map m1		# sets the map name to m1 unless explicitly set with --name flag\n" +
			"use map --reset 	# resets the behaviour",
	}
	// assign subcommands
	for _, sc := range []string{"map"} {
		tmp := cobra.Command{
			Use:   fmt.Sprintf("%s {defaultName | --reset}", sc),
			Short: fmt.Sprintf("set default name for %s commands", sc),
			Example: fmt.Sprintf("use %s m1\n"+
				"use %s --reset", sc, sc),
			RunE: func(cmd *cobra.Command, args []string) error {
				persister := common.PersisterFromContext(cmd.Context())
				if cmd.Flag("reset").Changed {
					persister.Reset(sc)
					return nil
				}
				if len(args) == 0 {
					return cmd.Help()
				}
				if len(args) > 1 {
					cmd.Println("Provide %s name between \"\" quotes if it contains white space", sc)
					return nil
				}
				persister.Set(sc, args[0])
				return nil
			},
		}
		tmp.Flags().Bool("reset", false, "unsets the default name for the type")
		cmd.AddCommand(&tmp)
	}
	return &cmd
}
