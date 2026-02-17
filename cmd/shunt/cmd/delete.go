package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a rule from the NATS KV bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Printf("Delete rule '%s' from KV bucket? [y/N] ", key)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled")
				return nil
			}
		}

		nc, err := connectNATS(cmd)
		if err != nil {
			return err
		}
		defer nc.Close()

		kv, err := openKVBucket(cmd, nc)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
		defer cancel()

		if err := kv.Delete(ctx, key); err != nil {
			return fmt.Errorf("failed to delete key '%s': %w", key, err)
		}

		fmt.Printf("Deleted '%s'\n", key)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}
