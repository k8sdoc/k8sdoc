package filters

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/user/k8sdoc/pkg/analyzer"
)
var FiltersCmd = &cobra.Command{
	Use:   "filters",
	Short: "Manage analyzer filters",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available analyzer filters",
	Run: func(cmd *cobra.Command, args []string) {
		core, kdoctor, additional := analyzer.ListFilters()
		sort.Strings(core)
		sort.Strings(kdoctor)
		sort.Strings(additional)

		fmt.Println("\n── Core Analyzers (always enabled) ──────────────────")
		for _, f := range core {
			fmt.Printf("  %-30s\n", f)
		}

		fmt.Println("\n── kdoctor Analyzers (enabled when CRDs present) ────")
		for _, f := range kdoctor {
			fmt.Printf("  %-30s\n", f)
		}

		fmt.Println("\n── Additional Analyzers (enable via --filter) ────────")
		for _, f := range additional {
			fmt.Printf("  %-30s\n", f)
		}

		fmt.Println("\nUsage: k8sdoc analyze --filter Pod,NetReach,AppHttpHealthy")
	},
}

func init() {
	FiltersCmd.AddCommand(listCmd)
}
