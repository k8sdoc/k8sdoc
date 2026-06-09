package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/k8sdoc/pkg/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var ProbeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Manage and inspect kdoctor probe tasks",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all kdoctor probe tasks and their last round status",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := newClient()
		if err != nil {
			color.Red("Error: %v", err)
			os.Exit(1)
		}

		ctx := context.Background()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "KIND\tNAME\tFINISH\tDONE_ROUND\tLAST_STATUS")
		fmt.Fprintln(w, "────\t────\t──────\t──────────\t───────────")

		kinds := map[string]schema.GroupVersionResource{
			"NetReach":       {Group: "kdoctor.io", Version: "v1beta1", Resource: "netreaches"},
			"AppHttpHealthy": {Group: "kdoctor.io", Version: "v1beta1", Resource: "apphttphealthies"},
			"Netdns":         {Group: "kdoctor.io", Version: "v1beta1", Resource: "netdnses"},
		}

		for kind, gvr := range kinds {
			list, err := client.GetDynamicClient().Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, item := range list.Items {
				name := item.GetName()
				statusRaw, _ := json.Marshal(item.Object["status"])
				var status struct {
					Finish          bool   `json:"finish"`
					DoneRound       *int64 `json:"doneRound"`
					LastRoundStatus string `json:"lastRoundStatus"`
				}
				_ = json.Unmarshal(statusRaw, &status)
				doneRound := int64(0)
				if status.DoneRound != nil {
					doneRound = *status.DoneRound
				}
				lastStatus := status.LastRoundStatus
				if lastStatus == "" {
					lastStatus = "unknown"
				}
				finish := "false"
				if status.Finish {
					finish = color.GreenString("true")
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", kind, name, finish, doneRound, lastStatus)
			}
		}
		_ = w.Flush()
	},
}

var getCmd = &cobra.Command{
	Use:   "get [task-name]",
	Short: "Get the latest round report for a kdoctor task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskName := args[0]
		client, err := newClient()
		if err != nil {
			color.Red("Error: %v", err)
			os.Exit(1)
		}

		ctx := context.Background()
		gvr := schema.GroupVersionResource{
			Group:   "system.kdoctor.io",
			Version: "v1beta1",
			Resource: "kdoctorreports",
		}
		report, err := client.GetDynamicClient().Resource(gvr).Get(ctx, taskName, metav1.GetOptions{})
		if err != nil {
			color.Red("Report not found for task %q: %v", taskName, err)
			os.Exit(1)
		}

		raw, _ := json.MarshalIndent(report.Object, "", "  ")
		fmt.Println(string(raw))
	},
}

func init() {
	ProbeCmd.AddCommand(listCmd)
	ProbeCmd.AddCommand(getCmd)
}

func newClient() (*kubernetes.Client, error) {
	return kubernetes.NewClient(
		viper.GetString("kubecontext"),
		viper.GetString("kubeconfig"),
	)
}
