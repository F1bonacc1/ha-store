package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dir",
		Short: "Manage directories",
	}

	cmd.AddCommand(newDirCreateCmd())
	cmd.AddCommand(newDirRmCmd())
	cmd.AddCommand(newDirExtractCmd())

	return cmd
}

func newDirCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <remote-path>",
		Short: "Create an empty directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.CreateDir(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", args[0])
			return nil
		},
	}
}

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls <remote-path>",
		Short: "List directory contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			files, err := c.ListDir(args[0])
			if err != nil {
				return err
			}
			for _, f := range files {
				fmt.Fprintln(cmd.OutOrStdout(), f)
			}
			return nil
		},
	}
}

func newDirRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <remote-path>",
		Short: "Delete a directory and its contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteDir(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", args[0])
			return nil
		},
	}
}

func newDirExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract <remote-path> <archive-file>",
		Short: "Upload and extract an archive into a directory",
		Long:  "Supported types: zip, tgz, targz, zst, 7z, 7zip",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			archiveType, _ := cmd.Flags().GetString("type")
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.ExtractArchive(args[0], args[1], archiveType); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Extracted %s into %s\n", args[1], args[0])
			return nil
		},
	}

	cmd.Flags().StringP("type", "t", "tgz", "archive type (zip, tgz, targz, zst, 7z, 7zip)")

	return cmd
}
