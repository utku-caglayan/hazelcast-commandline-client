package versioncmd

import (
	"fmt"
	"github.com/hazelcast/hazelcast-commandline-client/internal"
	"github.com/hazelcast/hazelcast-go-client"
	"github.com/spf13/cobra"
	"runtime"
)

func New() *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "Prints the version information.",
		Long:  `Prints the version of CLC and the related dependency versions including Go and Hazelcast Go Client.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {

			//cmdd := exec.Command("git", "rev-parse", "HEAD")
			//output, err := cmdd.Output()

			//if err != nil {
			//	fmt.Println(err.Error())
			//	return
			//}

			fmt.Printf("Command Line Client version: %s\n", internal.ClientVersion)
			fmt.Printf("Latest Git commit hash: %s\n", internal.CommitSHA)
			fmt.Printf("Go Client version: %s\n", hazelcast.ClientVersion)
			fmt.Printf("Go version: %s\n", runtime.Version())
		},
	}

	return &cmd
}
