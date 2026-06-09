package analyze

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/user/k8sdoc/pkg/analysis"
)

var (
	explain        bool
	backend        string
	output         string
	filters        []string
	language       string
	namespace      string
	labelSelector  string
	maxConcurrency int
)

// AnalyzeCmd is the primary diagnostic command
var AnalyzeCmd = &cobra.Command{
	Use:     "analyze",
	Aliases: []string{"analyse"},
	Short:   "Detect and diagnose issues in your Kubernetes cluster",
	Long: `analyze runs all k8sdoc analyzers (K8s resource state + kdoctor probe results)
and optionally calls a local AI model (Qwen via Ollama) to explain each issue
and suggest step-by-step kubectl fixes.

Examples:
  # Detect issues in all namespaces (no AI explanation)
  k8sdoc analyze

  # Detect + explain using local Ollama/Qwen
  k8sdoc analyze --explain

  # Scope to a namespace
  k8sdoc analyze --namespace production --explain

  # Only run kdoctor NetReach analyzer
  k8sdoc analyze --filter NetReach --explain

  # JSON output for CI
  k8sdoc analyze --output json`,

	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := analysis.NewAnalysis(
			backend, language, filters,
			namespace, labelSelector,
			explain, maxConcurrency,
		)
		if err != nil {
			color.Red("Error: %v", err)
			os.Exit(1)
		}
		defer cfg.Close()

		// Collect kdoctor probe results before running analyzers
		cfg.CollectProbeResults()

		// Run all applicable analyzers in parallel
		cfg.RunAnalysis()

		// If --explain, call the local AI for each result
		if explain {
			if err := cfg.GetAIResults(output); err != nil {
				color.Red("AI diagnosis error: %v", err)
				os.Exit(1)
			}
		}

		out, err := cfg.PrintOutput(output)
		if err != nil {
			color.Red("Output error: %v", err)
			os.Exit(1)
		}
		fmt.Print(string(out))
	},
}

func init() {
	AnalyzeCmd.Flags().BoolVarP(&explain, "explain", "e", false,
		"Call the local AI to explain each issue and suggest fixes")
	AnalyzeCmd.Flags().StringVarP(&backend, "backend", "b", "",
		"AI backend to use (default: from config, usually 'ollama')")
	AnalyzeCmd.Flags().StringVarP(&output, "output", "o", "text",
		"Output format: text | json")
	AnalyzeCmd.Flags().StringSliceVarP(&filters, "filter", "f", []string{},
		"Analyzer filter(s) to run, e.g. Pod,NetReach,AppHttpHealthy")
	AnalyzeCmd.Flags().StringVarP(&language, "language", "l", "English",
		"Language for AI explanations")
	AnalyzeCmd.Flags().StringVarP(&namespace, "namespace", "n", "",
		"Kubernetes namespace to analyze (default: all namespaces)")
	AnalyzeCmd.Flags().StringVarP(&labelSelector, "selector", "L", "",
		"Label selector to filter resources, e.g. app=myapp")
	AnalyzeCmd.Flags().IntVarP(&maxConcurrency, "max-concurrency", "m", 10,
		"Maximum concurrent analyzer goroutines")
}
