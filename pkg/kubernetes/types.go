package kubernetes

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	Client        kubernetes.Interface
	CtrlClient    ctrl.Client
	Config        *rest.Config
	DynamicClient dynamic.Interface
}

func (c *Client) GetClient() kubernetes.Interface {
	return c.Client
}

func (c *Client) GetCtrlClient() ctrl.Client {
	return c.CtrlClient
}

func (c *Client) GetDynamicClient() dynamic.Interface {
	return c.DynamicClient
}
