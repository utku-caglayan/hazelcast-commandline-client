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
package rootcmd

import (
	"errors"
	"fmt"
	"github.com/hazelcast/hazelcast-commandline-client/connwizardcmd"
	"os"
	"strings"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/spf13/cobra"

	"github.com/hazelcast/hazelcast-commandline-client/clustercmd"
	"github.com/hazelcast/hazelcast-commandline-client/config"
	hzcerrors "github.com/hazelcast/hazelcast-commandline-client/errors"
	"github.com/hazelcast/hazelcast-commandline-client/listobjectscmd"
	"github.com/hazelcast/hazelcast-commandline-client/sqlcmd"
	fakeDoor "github.com/hazelcast/hazelcast-commandline-client/types/fakedoorcmd"
	"github.com/hazelcast/hazelcast-commandline-client/types/mapcmd"
	"github.com/hazelcast/hazelcast-commandline-client/versioncmd"
)

// New initializes root command for non-interactive mode
func New(cnfg *hazelcast.Config, isInteractiveInvocation bool) (*cobra.Command, *config.GlobalFlagValues) {
	root := NewWithoutPersistentFlags(cnfg, isInteractiveInvocation)
	var flags config.GlobalFlagValues
	assignPersistentFlags(root, &flags)
	return root, &flags
}

// NewWithoutPersistentFlags initializes root command without the persistent flags
func NewWithoutPersistentFlags(cnfg *hazelcast.Config, isInteractiveInvocation bool) *cobra.Command {
	root := &cobra.Command{
		Use:   "hzc {cluster | map | sql | version | help} [--address address | --cloud-token token | --cluster-name name | --config config]",
		Short: "Hazelcast command-line client",
		Long:  "Hazelcast command-line client connects your command-line to a Hazelcast cluster",
		Example: `hzc # starts an interactive shell 🚀
hzc map put --name my-map --key hello --value world # put entry into map directly
hzc sql # starts the SQL Browser
hzc help # print help`,
		// Handle errors explicitly
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Make sure the command receives non-nil configuration
			if cnfg == nil {
				return errors.New("missing configuration")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// print usage on flag errors
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		_ = cmd.Help()
		return hzcerrors.FlagError{err}
	})
	root.CompletionOptions.DisableDefaultCmd = true
	// This is used to generate completion scripts
	if mode := os.Getenv("MODE"); strings.EqualFold(mode, "dev") {
		root.CompletionOptions.DisableDefaultCmd = false
	}
	root.AddCommand(subCommands(cnfg, isInteractiveInvocation)...)
	return root
}

func subCommands(config *hazelcast.Config, isInteractiveInvocation bool) []*cobra.Command {
	cmds := []*cobra.Command{
		clustercmd.New(config),
		mapcmd.New(config, isInteractiveInvocation),
		sqlcmd.New(config),
		versioncmd.New(),
		listobjectscmd.New(config),
		connwizardcmd.New(),
	}
	fds := []fakeDoor.FakeDoor{
		{Name: "List", IssueNum: 48},
		{Name: "Queue", IssueNum: 49},
		{Name: "MultiMap", IssueNum: 50},
		{Name: "ReplicatedMap", IssueNum: 51},
		{Name: "Set", IssueNum: 52},
		{Name: "Topic", IssueNum: 53},
	}
	for _, fd := range fds {
		cmds = append(cmds, fakeDoor.NewFakeCommand(fd))
	}
	return cmds
}

// assignPersistentFlags assigns top level flags to command
func assignPersistentFlags(cmd *cobra.Command, flags *config.GlobalFlagValues) {
	cmd.PersistentFlags().StringVarP(&flags.CfgFile, "config", "c", config.DefaultConfigPath(), fmt.Sprintf("config file, only supports yaml for now"))
	cmd.PersistentFlags().StringVarP(&flags.Address, "address", "a", "", fmt.Sprintf("addresses of the instances in the cluster (default is %s)", config.DefaultClusterAddress))
	cmd.PersistentFlags().StringVar(&flags.Cluster, "cluster-name", "", fmt.Sprintf("name of the cluster that contains the instances (default is %s)", config.DefaultClusterName))
	cmd.PersistentFlags().StringVar(&flags.Token, "cloud-token", "", "your Hazelcast Viridian token")
	cmd.PersistentFlags().BoolVar(&flags.Verbose, "verbose", false, "verbose output")
	cmd.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "disable colors")
	cmd.PersistentFlags().BoolVar(&flags.NoAutocompletion, "no-completion", false, "disable completion [interactive mode]")
	cmd.PersistentFlags().StringVar(&flags.LogFile, "log-file", "", "direct logs to specified file")
	cmd.PersistentFlags().StringVar(&flags.LogLevel, "log-level", "", fmt.Sprintf("(default 'error') write logs with the specified or higher level of importance. Log levels: %s", strings.Join(config.ValidLogLevels, ", ")))
	cmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return config.ValidLogLevels, cobra.ShellCompDirectiveDefault
	})

}

func init() {
	cobra.MousetrapHelpText = ""
}
