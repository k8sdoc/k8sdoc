package auth

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/k8sdoc/pkg/ai"
)

var (
	addBackend     string
	addModel       string
	addBaseURL     string
	addTemperature float32
	addTopP        float32
	addMaxTokens   int
	setDefault     bool
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add or update an AI backend provider",
	Long: `Configure a local AI backend (Ollama + Qwen).

Examples:
  # Configure Ollama with Qwen
  k8sdoc auth add --backend ollama \
    --baseurl http://localhost:11434 \
    --model qwen2.5-coder:14b-instruct-q4_K_M`,

	Run: func(cmd *cobra.Command, args []string) {
		if addBackend == "" {
			addBackend = "ollama"
		}

		// Validate backend name
		valid := false
		for _, b := range ai.Backends {
			if b == addBackend {
				valid = true
				break
			}
		}
		if !valid {
			color.Red("Unknown backend: %s. Available: %v", addBackend, ai.Backends)
			os.Exit(1)
		}

		provider := ai.AIProvider{
			Name:        addBackend,
			Model:       addModel,
			BaseURL:     addBaseURL,
			Temperature: addTemperature,
			TopP:        addTopP,
			MaxTokens:   addMaxTokens,
		}

		var configAI ai.AIConfiguration
		_ = viper.UnmarshalKey("ai", &configAI)

		found := false
		for i, p := range configAI.Providers {
			if p.Name == addBackend {
				configAI.Providers[i] = provider
				found = true
				break
			}
		}
		if !found {
			configAI.Providers = append(configAI.Providers, provider)
		}

		if setDefault || configAI.DefaultProvider == "" {
			configAI.DefaultProvider = addBackend
		}

		viper.Set("ai", configAI)
		if err := viper.WriteConfig(); err != nil {
			if err := viper.SafeWriteConfig(); err != nil {
				color.Red("Failed to write config: %v", err)
				os.Exit(1)
			}
		}

		color.Green("✓ Configured backend: %s (model: %s baseurl: %s)", addBackend, addModel, addBaseURL)
		fmt.Printf("  Test: ollama list\n  Then: k8sdoc serve\n")
	},
}

func init() {
	addCmd.Flags().StringVarP(&addBackend, "backend", "b", "ollama",
		fmt.Sprintf("AI backend. Available: %v", ai.Backends))
	addCmd.Flags().StringVarP(&addModel, "model", "m", "qwen2.5-coder:14b-instruct-q4_K_M",
		"Ollama model name")
	addCmd.Flags().StringVar(&addBaseURL, "baseurl", "http://localhost:11434",
		"Ollama server URL")
	addCmd.Flags().Float32Var(&addTemperature, "temperature", 0.1,
		"Sampling temperature (lower = more deterministic)")
	addCmd.Flags().Float32Var(&addTopP, "top-p", 0.9, "Top-p nucleus sampling")
	addCmd.Flags().IntVar(&addMaxTokens, "max-tokens", 4096, "Max tokens per response")
	addCmd.Flags().BoolVar(&setDefault, "default", true, "Set as default provider")
}
