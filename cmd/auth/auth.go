package auth

import "github.com/spf13/cobra"

var AuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage AI backend provider configuration",
}

func init() {
	AuthCmd.AddCommand(addCmd)
	AuthCmd.AddCommand(listCmd)
	AuthCmd.AddCommand(removeCmd)
}
