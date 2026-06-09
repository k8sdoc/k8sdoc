// Orchestrates the full pipeline:
//  1. Collect K8s resource state + kdoctor probe results
//  2. Run all relevant analyzers in parallel
//  3. (Optionally) call the local AI to generate per-result diagnoses
//  4. Format and return output
package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/viper"
	"github.com/user/k8sdoc/pkg/ai"
	"github.com/user/k8sdoc/pkg/analyzer"
	"github.com/user/k8sdoc/pkg/collector"
	"github.com/user/k8sdoc/pkg/common"
	"github.com/user/k8sdoc/pkg/kubernetes"
)

// Struct holds config and accumulates results
type Analysis struct {
	Context        context.Context
	Client         *kubernetes.Client
	AIClient       ai.IAI
	Filters        []string
	Results        []common.Result
	ProbeResults   []common.ProbeResult
	Errors         []string
	Namespace      string
	LabelSelector  string
	Language       string
	Explain        bool
	MaxConcurrency int
	Verbose        bool
}

type JSONOutput struct {
	Provider string               `json:"provider"`
	Status   common.AnalysisStatus `json:"status"`
	Problems int                  `json:"problems"`
	Errors   []string             `json:"errors,omitempty"`
	Results  []common.Result      `json:"results"`
}

func NewAnalysis(
	backend, language string,
	filters []string,
	namespace, labelSelector string,
	explain bool,
	maxConcurrency int,
) (*Analysis, error) {
	kubecontext := viper.GetString("kubecontext")
	kubeconfig := viper.GetString("kubeconfig")
	verbose := viper.GetBool("verbose")

	client, err := kubernetes.NewClient(kubecontext, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("initialising kubernetes client: %w", err)
	}

	a := &Analysis{
		Context:        context.Background(),
		Client:         client,
		Filters:        filters,
		Namespace:      namespace,
		LabelSelector:  labelSelector,
		Language:       language,
		Explain:        explain,
		MaxConcurrency: maxConcurrency,
		Verbose:        verbose,
	}

	if !explain {
		a.AIClient = &ai.NoOpAIClient{}
		return a, nil
	}

	var configAI ai.AIConfiguration
	if err := viper.UnmarshalKey("ai", &configAI); err != nil {
		return nil, err
	}
	if len(configAI.Providers) == 0 {
		return nil, fmt.Errorf("no AI provider configured - run: k8sdoc auth add --backend ollama")
	}

	if backend == "" {
		backend = configAI.DefaultProvider
	}
	if backend == "" {
		backend = "ollama"
	}

	var aiProvider ai.AIProvider
	for _, p := range configAI.Providers {
		if p.Name == backend {
			aiProvider = p
			break
		}
	}
	if aiProvider.Name == "" {
		return nil, fmt.Errorf("AI provider %q not found in config", backend)
	}

	aiClient := ai.NewClient(aiProvider.Name)
	if err := aiClient.Configure(&aiProvider); err != nil {
		return nil, fmt.Errorf("configuring AI client: %w", err)
	}
	a.AIClient = aiClient

	if verbose {
		fmt.Printf("✓ AI: %s model=%s\n", aiProvider.Name, aiProvider.Model)
	}
	return a, nil
}

// NewAnalysisWithClient creates an Analysis reusing existing client connections
// Used by chat engine to avoid creating duplicate connections
func NewAnalysisWithClient(client *kubernetes.Client, aiClient ai.IAI, namespace string) (*Analysis, error) {
	return &Analysis{
		Context:        context.Background(),
		Client:         client,
		AIClient:       aiClient,
		Namespace:      namespace,
		Language:       "English",
		MaxConcurrency: 10,
		Verbose:        false,
	}, nil
}

func (a *Analysis) CollectProbeResults() {
	kc := collector.NewKdoctorCollector(a.Client, a.Namespace)
	probes, err := kc.Collect(a.Context)
	if err != nil || probes == nil {
		return
	}
	a.ProbeResults = probes
}

func (a *Analysis) RunAnalysis() {
	analyzers := analyzer.GetAnalyzers(a.Filters)
	semaphore := make(chan struct{}, a.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, ana := range analyzers {
		wg.Add(1)
		go func(name string, ana common.IAnalyzer) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			input := common.Analyzer{
				Client:        a.Client,
				Context:       a.Context,
				Namespace:     a.Namespace,
				LabelSelector: a.LabelSelector,
				ProbeResults:  a.ProbeResults,
			}
			results, err := ana.Analyze(input)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				a.Errors = append(a.Errors, fmt.Sprintf("[%s] %s", name, err))
			} else {
				a.Results = append(a.Results, results...)
			}
		}(name, ana)
	}
	wg.Wait()
}

func (a *Analysis) GetAIResults(output string) error {
	if len(a.Results) == 0 {
		return nil
	}

	var bar *progressbar.ProgressBar
	if output != "json" {
		bar = progressbar.Default(int64(len(a.Results)))
	}

	for i, res := range a.Results {
		if bar != nil {
			bar.Describe(fmt.Sprintf("Diagnosing %s/%s", res.Kind, res.Name))
		}

		var texts []string
		for _, f := range res.Error {
			texts = append(texts, f.Text)
		}
		errorText := strings.Join(texts, "\n")

		probeCtx := a.buildProbeContext(res)
		promptTemplate, ok := ai.PromptMap[res.Kind]
		if !ok {
			promptTemplate = ai.PromptMap["default"]
		}

		var prompt string
		if probeCtx != "" {
			prompt = fmt.Sprintf(promptTemplate, a.Language, errorText, probeCtx)
		} else {
			prompt = fmt.Sprintf(ai.PromptMap["default"], a.Language, errorText)
		}

		details, err := a.AIClient.GetCompletion(a.Context, prompt)
		if err != nil {
			if bar != nil {
				_ = bar.Exit()
			}
			return fmt.Errorf("AI completion failed for %s/%s: %w", res.Kind, res.Name, err)
		}

		a.Results[i].Details = details
		if bar != nil {
			_ = bar.Add(1)
		}
	}
	return nil
}

func (a *Analysis) buildProbeContext(res common.Result) string {
	if res.ProbeRef == "" {
		return ""
	}
	for _, pr := range a.ProbeResults {
		if pr.TaskName == res.ProbeRef {
			return fmt.Sprintf(
				"Task: %s | Kind: %s | Round: %d | Result: %s\n"+
					"Success Rate: %.1f%% | P95 Latency: %dms | Duration: %s\n"+
					"Failed Pods/Nodes: %s\nFailure Reason: %s",
				pr.TaskName, pr.TaskKind, pr.RoundNumber, pr.Result,
				pr.SuccessRate*100, pr.LatencyP95Ms, pr.RoundDuration,
				strings.Join(pr.FailedNodes, ", "),
				pr.FailedReason,
			)
		}
	}
	return ""
}

func (a *Analysis) PrintOutput(format string) ([]byte, error) {
	status := common.StateOK
	if len(a.Results) > 0 {
		status = common.StateProblemDetected
	}

	switch format {
	case "json":
		out := JSONOutput{
			Provider: a.AIClient.GetName(),
			Status:   status,
			Problems: len(a.Results),
			Errors:   a.Errors,
			Results:  a.Results,
		}
		return json.MarshalIndent(out, "", "  ")

	default:
		var sb strings.Builder
		if len(a.Results) == 0 {
			sb.WriteString(color.GreenString("✓ No issues detected\n"))
			return []byte(sb.String()), nil
		}

		sb.WriteString(fmt.Sprintf("\n%s %d issue(s) detected\n\n",
			color.RedString("✗"), len(a.Results)))

		for _, res := range a.Results {
			sb.WriteString(color.CyanString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"))
			sb.WriteString(fmt.Sprintf("%s %s/%s\n", color.YellowString("►"), res.Kind, res.Name))
			if res.ProbeRef != "" {
				sb.WriteString(fmt.Sprintf("  Probe: %s\n", color.MagentaString(res.ProbeRef)))
			}
			sb.WriteString("\n")
			for _, f := range res.Error {
				sb.WriteString(fmt.Sprintf("  %s %s\n", color.RedString("•"), f.Text))
			}
			if res.Details != "" {
				sb.WriteString("\n")
				for _, line := range strings.Split(strings.TrimSpace(res.Details), "\n") {
					sb.WriteString(fmt.Sprintf("  %s\n", line))
				}
			}
			sb.WriteString("\n")
		}

		if len(a.Errors) > 0 {
			sb.WriteString(color.YellowString("\nWarnings:\n"))
			for _, e := range a.Errors {
				sb.WriteString(fmt.Sprintf("  ! %s\n", e))
			}
		}

		sb.WriteString(fmt.Sprintf("\nAnalysis completed at %s\n", time.Now().Format("2006-01-02 15:04:05")))
		return []byte(sb.String()), nil
	}
}

func (a *Analysis) Close() {
	if a.AIClient != nil {
		a.AIClient.Close()
	}
}
