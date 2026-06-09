package auth

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/k8sdoc/pkg/ai"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured AI providers",
	Run: func(cmd *cobra.Command, args []string) {
		var configAI ai.AIConfiguration
		if err := viper.UnmarshalKey("ai", &configAI); err != nil {
			color.Red("Error reading config: %v", err)
			return
		}
		if len(configAI.Providers) == 0 {
			fmt.Println("No providers configured. Run: k8sdoc auth add --backend ollama")
			return
		}
		fmt.Printf("%-12s %-45s %-10s %-8s\n", "BACKEND", "MODEL", "BASE_URL", "DEFAULT")
		fmt.Printf("%-12s %-45s %-10s %-8s\n",
			"────────────", "─────────────────────────────────────────────", "──────────", "───────")
		for _, p := range configAI.Providers {
			def := ""
			if p.Name == configAI.DefaultProvider {
				def = color.GreenString("✓")
			}
			fmt.Printf("%-12s %-45s %-10s %-8s\n", p.Name, p.Model, p.BaseURL, def)
		}
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove [backend-name]",
	Short: "Remove a configured AI provider",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		var configAI ai.AIConfiguration
		_ = viper.UnmarshalKey("ai", &configAI)
		updated := configAI.Providers[:0]
		for _, p := range configAI.Providers {
			if p.Name != name {
				updated = append(updated, p)
			}
		}
		configAI.Providers = updated
		if configAI.DefaultProvider == name && len(updated) > 0 {
			configAI.DefaultProvider = updated[0].Name
		}
		viper.Set("ai", configAI)
		_ = viper.WriteConfig()
		color.Green("✓ Removed: %s", name)
	},
}
