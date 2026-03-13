package cmd

import (
	"fmt"
	"os"

	"github.com/f1bonacc1/ha-store/stctl/client"
	"github.com/spf13/cobra"
)

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Manage files",
	}

	cmd.AddCommand(newFilePutCmd())
	cmd.AddCommand(newFileGetCmd())
	cmd.AddCommand(newFileRmCmd())

	return cmd
}

func newFilePutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "put <remote-path> <local-file>",
		Short: "Upload a file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			perms, _ := cmd.Flags().GetString("permissions")
			owner, _ := cmd.Flags().GetString("owner")
			group, _ := cmd.Flags().GetString("group")
			stats, err := c.PutFileWithOptions(args[0], args[1], client.PutFileOptions{
				Permissions: perms,
				Owner:       owner,
				Group:       group,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %s (%s)\n", args[0], stats)
			return nil
		},
	}

	cmd.Flags().StringP("permissions", "m", "", "file permissions in octal (e.g. 0644)")
	cmd.Flags().StringP("owner", "o", "", "file owner")
	cmd.Flags().StringP("group", "g", "", "file group")

	return cmd
}

func newFileGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <remote-path> [local-file]",
		Short: "Download a file",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if len(args) == 2 {
				stats, err := c.GetFile(args[0], args[1])
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s -> %s (%s)\n", args[0], args[1], stats)
			} else {
				if _, err := c.GetFileToWriter(args[0], os.Stdout); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newFileRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <remote-path>",
		Short: "Delete a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteFile(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", args[0])
			return nil
		},
	}
}
