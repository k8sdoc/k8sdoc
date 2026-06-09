package kubernetes

import (
	"fmt"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)


func NewClient(kubecontext, kubeconfig string) (*Client, error) {
	var config *rest.Config
	var err error

	config, err = rest.InClusterConfig()
	if kubeconfig != "" || err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfig != "" {
			loadingRules.ExplicitPath = kubeconfig
		}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			&clientcmd.ConfigOverrides{CurrentContext: kubecontext},
		)
		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("building kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	ctrlClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("creating ctrl client: %w", err)
	}

	return &Client{
		Client:        clientset,
		CtrlClient:    ctrlClient,
		Config:        config,
		DynamicClient: dynamicClient,
	}, nil
}
