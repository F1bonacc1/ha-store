package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/f1bonacc1/ha-store/stctl/client"
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
	cmd := &cobra.Command{
		Use:   "ls <remote-path>",
		Short: "List directory contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			long, _ := cmd.Flags().GetBool("long")
			c, err := newClient()
			if err != nil {
				return err
			}

			if long {
				return listLong(cmd, c, args[0])
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

	cmd.Flags().BoolP("long", "l", false, "use a long listing format")
	return cmd
}

func listLong(cmd *cobra.Command, c *client.Client, path string) error {
	files, err := c.ListDirDetailed(path)
	if err != nil {
		return err
	}
	for _, f := range files {
		perms := formatPermissions(f.Permissions, strings.HasSuffix(f.Name, "/"))
		modTime := formatModTime(f.ModTime)
		size := formatSize(f.Size)
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %-8s %-8s %8s  %s  %s\n",
			perms, f.Owner, f.Group, size, modTime, f.Name)
	}
	return nil
}

// formatPermissions converts an octal string like "0755" to symbolic like "-rwxr-xr-x".
func formatPermissions(octal string, isDir bool) string {
	mode, err := strconv.ParseUint(octal, 8, 32)
	if err != nil {
		mode = 0o644
	}

	var buf [10]byte
	if isDir {
		buf[0] = 'd'
	} else {
		buf[0] = '-'
	}

	const rwx = "rwx"
	for i := 0; i < 9; i++ {
		if mode&(1<<uint(8-i)) != 0 {
			buf[1+i] = rwx[i%3]
		} else {
			buf[1+i] = '-'
		}
	}
	return string(buf[:])
}

func formatModTime(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Format("Jan _2 15:04")
}

func formatSize(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d", b)
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
			if archiveType == "" {
				archiveType = detectArchiveType(args[1])
			}
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

	cmd.Flags().StringP("type", "t", "", "archive type (zip, tgz, targz, zst, 7z, 7zip); auto-detected from extension if omitted")

	return cmd
}

// detectArchiveType infers the server archive type from a filename's extension.
func detectArchiveType(filename string) string {
	lower := strings.ToLower(filename)
	switch filepath.Ext(lower) {
	case ".zip":
		return "zip"
	case ".tgz":
		return "tgz"
	case ".zst":
		if strings.HasSuffix(lower, ".tar.zst") {
			return "zst"
		}
		return "zst"
	case ".7z":
		return "7z"
	case ".gz":
		if strings.HasSuffix(lower, ".tar.gz") {
			return "tgz"
		}
	}
	return "tgz"
}
