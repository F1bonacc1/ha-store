package cmd

import (
	"github.com/f1bonacc1/ha-store/stctl/client"
	"github.com/spf13/cobra"
)

var (
	serverURL   string
	tlsCertFile string
	tlsKeyFile  string
	tlsCAFile   string
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "stctl",
		Short: "CLI client for ha-store",
		Long:  "stctl is a command-line tool to interact with an ha-store server.",
	}

	root.PersistentFlags().StringVarP(&serverURL, "server", "s", "http://localhost:8090", "ha-store server URL")
	root.PersistentFlags().StringVar(&tlsCertFile, "tls-cert", "", "client certificate file for mTLS")
	root.PersistentFlags().StringVar(&tlsKeyFile, "tls-key", "", "client private key file for mTLS")
	root.PersistentFlags().StringVar(&tlsCAFile, "tls-ca", "", "CA certificate file for verifying the server")

	root.AddCommand(newFileCmd())
	root.AddCommand(newDirCmd())
	root.AddCommand(newLsCmd())

	return root
}

func newClient() (*client.Client, error) {
	if tlsCertFile != "" && tlsKeyFile != "" && tlsCAFile != "" {
		return client.NewWithTLS(serverURL, tlsCertFile, tlsKeyFile, tlsCAFile)
	}
	return client.New(serverURL), nil
}
