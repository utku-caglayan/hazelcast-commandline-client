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
	"github.com/spf13/cobra"

	"github.com/hazelcast/hazelcast-commandline-client/commands/common"
)

func New() *cobra.Command {
	cmd := cobra.Command{
		Use:   "use [--reset]",
		Short: "sets default name for all of the data structures such as map, queue, topic...",
		Example: "use m1			# sets the default name to m1 unless explicitly set with --name flag\n" +
			"map get --key k1 	# \"--name m1\" is inferred unless set explicitly\n" +
			"use --reset		# resets the behaviour",
		RunE: func(cmd *cobra.Command, args []string) error {
			persister := common.PersisterFromContext(cmd.Context())
			if cmd.Flag("reset").Changed {
				persister.Reset("name")
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) > 1 {
				cmd.Println("Provide default name between \"\" quotes if it contains white space")
				return nil
			}
			persister.Set("name", args[0])
			return nil
		}}
	cmd.Flags().Bool("reset", false, "unsets the default name for the type")
	return &cmd
}
