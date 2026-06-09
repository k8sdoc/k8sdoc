package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/k8sdoc/cmd/analyze"
	"github.com/user/k8sdoc/cmd/auth"
	"github.com/user/k8sdoc/cmd/filters"
	"github.com/user/k8sdoc/cmd/probe"
	"github.com/user/k8sdoc/cmd/serve"
)

var (
	cfgFile     string
	kubecontext string
	kubeconfig  string
	verbose     bool
	Version     string
	Commit      string
	Date        string
)

var rootCmd = &cobra.Command{
	Use:   "k8sdoc",
	Short: "AI-powered Kubernetes diagnostics using local models",
	Long: `k8sdoc combines kdoctor's active network probing with a locally-running
AI model (Qwen via Ollama) to diagnose, explain, and suggest fixes for
Kubernetes cluster issues without sending data to external APIs.`,
}

func Execute(v, c, d string) {
	Version = v
	Commit = c
	Date = d
	viper.Set("Version", Version)
	viper.Set("Commit", Commit)
	viper.Set("Date", Date)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.k8sdoc/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&kubecontext, "kubecontext", "", "Kubernetes context to use")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose/debug output")

	_ = viper.BindPFlag("kubecontext", rootCmd.PersistentFlags().Lookup("kubecontext"))
	_ = viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	rootCmd.AddCommand(analyze.AnalyzeCmd)
	rootCmd.AddCommand(auth.AuthCmd)
	rootCmd.AddCommand(filters.FiltersCmd)
	rootCmd.AddCommand(probe.ProbeCmd)
	rootCmd.AddCommand(serve.ServeCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		configDir := filepath.Join(home, ".k8sdoc")
		_ = os.MkdirAll(configDir, 0o755)
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print k8sdoc version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("k8sdoc %s (commit: %s, built: %s)\n", Version, Commit, Date)
	},
}
