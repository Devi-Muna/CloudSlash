package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps the k8s clientset
type Client struct {
	Clientset *kubernetes.Clientset
}

// NewClient attempts to load kubeconfig from home dir or in-cluster config
func NewClient() (*Client, error) {
	var config *rest.Config
	var err error

	// 1. Try In-Cluster Config (if running inside a pod)
	config, err = rest.InClusterConfig()
	if err != nil {
		// 2. Try Local Kubeconfig
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		} else {
			kubeconfig = os.Getenv("KUBECONFIG")
		}

		if kubeconfig == "" {
			return nil, fmt.Errorf("no kubeconfig found")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	return &Client{
		Clientset: clientset,
	}, nil
}
