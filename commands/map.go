package commands

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/types"
	"github.com/spf13/cobra"
)

var mapName string
var mapKey string
var mapValue string

var mapValueType string
var mapValueFile string

var mapCmd = &cobra.Command{
	Use:   "map {get | put} --name mapname --key keyname [--value-type type | --value-file file | --value value]",
	Short: "map operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	mapCmd.PersistentFlags().StringVarP(&mapName, "name", "m", "", "specify the map name")
	rootCmd.AddCommand(mapCmd)
}

func getMap(clientConfig *hazelcast.Config, mapName string) (*hazelcast.Map, error) {
	var client *hazelcast.Client
	var err error
	defer func() {
		obj := recover()
		if err, ok := obj.(error); ok {
			var addrErr *net.AddrError
			if errors.As(err, &addrErr) {
				log.Fatal(fmt.Errorf("given address is invalid: %s\n%s", addrErr.Addr, err))
			}
			log.Fatal(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if mapName == "" {
		return nil, errors.New("map name is required")
	}
	if clientConfig == nil {
		clientConfig = &hazelcast.Config{}
	}
	clientConfig.Cluster.ConnectionStrategy.Retry.InitialBackoff = types.Duration(1 * time.Second)
	client, err = hazelcast.StartNewClientWithConfig(ctx, *clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating the client: %w", err)
	}
	if result, err := client.GetMap(ctx, mapName); err != nil {
		return nil, err
	} else {
		return result, nil
	}
}

func decorateCommandWithKeyFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&mapKey, "key", "k", "", "key of the map")
	cmd.MarkFlagRequired("key")
}

func decorateCommandWithValueFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.StringVarP(&mapValue, "value", "v", "", "value of the map")
	flags.StringVarP(&mapValueType, "value-type", "t", "string", "type of the value, one of: string, json")
	flags.StringVarP(&mapValueFile, "value-file", "f", "", `path to the file that contains the value. Use "-" (dash) to read from stdin`)
	cmd.RegisterFlagCompletionFunc("value-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "string"}, cobra.ShellCompDirectiveDefault
	})
}