package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/k8sdoc/pkg/ai"
	"github.com/user/k8sdoc/pkg/analysis"
	"github.com/user/k8sdoc/pkg/chat"
	"github.com/user/k8sdoc/pkg/kubernetes"
)

var (
	port      int
	namespace string
)

// ServeCmd starts k8sdoc as a web server with a built-in chat UI.
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start k8sdoc web chat UI + API server",
	Long: `Starts the k8sdoc web server with a built-in chat interface.

Open http://localhost:8080 in your browser to chat with your cluster.

You can ask things like:
  "show me all pods"
  "why is my nginx pod crashing?"
  "get logs from pod my-app-xyz"
  "what events are happening in production namespace?"
  "diagnose all issues in the cluster"`,

	Run: func(cmd *cobra.Command, args []string) {
		// Load AI config
		var configAI ai.AIConfiguration
		if err := viper.UnmarshalKey("ai", &configAI); err != nil || len(configAI.Providers) == 0 {
			color.Red("No AI provider configured. Run: k8sdoc auth add --backend ollama")
			os.Exit(1)
		}
		provider := configAI.Providers[0]
		for _, p := range configAI.Providers {
			if p.Name == configAI.DefaultProvider {
				provider = p
				break
			}
		}

		aiClient := ai.NewClient(provider.Name)
		if err := aiClient.Configure(&provider); err != nil {
			color.Red("Failed to configure AI client: %v", err)
			os.Exit(1)
		}

		// Build kubernetes client
		k8sClient, err := kubernetes.NewClient(
			viper.GetString("kubecontext"),
			viper.GetString("kubeconfig"),
		)
		if err != nil {
			color.Red("Failed to connect to Kubernetes: %v", err)
			os.Exit(1)
		}

		// Build chat engine
		engine := chat.NewEngine(aiClient, k8sClient, namespace)

		// Register routes
		mux := http.NewServeMux()
		mux.HandleFunc("/", uiHandler)
		mux.HandleFunc("/healthz", healthHandler)
		mux.HandleFunc("/api/v1/chat", chatHandler(engine))
		mux.HandleFunc("/api/v1/analyze", analyzeHandler(k8sClient, aiClient))
		mux.HandleFunc("/api/v1/cluster/summary", summaryHandler(k8sClient))
		mux.HandleFunc("/api/v1/manifest", manifestHandler(engine))

		srv := &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 10 * time.Minute, // streaming can take time
		}

		color.Green("┌─────────────────────────────────────────┐")
		color.Green("│  k8sdoc Web UI                          │")
		color.Green("│  http://localhost:%d                  │", port)
		color.Green("│  AI: %s (%s)%s│", provider.Name, provider.Model,
			strings.Repeat(" ", max(0, 26-len(provider.Name)-len(provider.Model))))
		color.Green("└─────────────────────────────────────────┘")

		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				color.Red("Server error: %v", err)
				os.Exit(1)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		fmt.Println("\nServer stopped.")
	},
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}


type ChatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
	Namespace string `json:"namespace"`
}

func chatHandler(engine *chat.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.SessionID == "" {
			req.SessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
		}
		if req.Message == "" {
			http.Error(w, "message required", http.StatusBadRequest)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		writeSSE := func(chunk chat.StreamChunk) {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		engine.Chat(r.Context(), req.SessionID, req.Message, writeSSE)
	}
}


type AnalyzeRequest struct {
	Namespace string   `json:"namespace"`
	Filters   []string `json:"filters"`
	Explain   bool     `json:"explain"`
}

func analyzeHandler(client *kubernetes.Client, aiClient ai.IAI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var req AnalyzeRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		cfg, err := analysis.NewAnalysisWithClient(client, aiClient, req.Namespace)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cfg.CollectProbeResults()
		cfg.RunAnalysis()
		if req.Explain {
			_ = cfg.GetAIResults("json")
		}
		out, _ := cfg.PrintOutput("json")
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}
}

// Cluster summary handler

func summaryHandler(client *kubernetes.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tools := chat.NewK8sTools(client)
		summary, err := tools.GetClusterSummary(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"summary": summary})
	}
}


type ManifestRequest struct {
	SessionID string `json:"sessionId"`
	Filename  string `json:"filename"`
	Content   string `json:"content"`
}

func manifestHandler(engine *chat.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var req ManifestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.SessionID == "" {
			req.SessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
		}
		if req.Filename == "" {
			req.Filename = "manifest.yaml"
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		writeSSE := func(chunk chat.StreamChunk) {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		engine.AnalyzeManifest(r.Context(), req.SessionID, req.Content, req.Filename, writeSSE)
	}
}


func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}


func uiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(webUI))
}

func init() {
	ServeCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")
	ServeCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Default namespace (empty = all)")
}
