package openshiftcluster

import (
	"fmt"
	v1 "github.com/openshift/api/config/v1"
	client "github.com/openshift/client-go/config/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"log"
)

// NewClient uses kubeconfig to create a client for existing openshift cluster
// Return openshift Client
func NewClient(kubeConfigPath *string) (*client.Clientset, error) {
	log.Println("kubeconfig source: ", *kubeConfigPath)
	c, err := clientcmd.BuildConfigFromFlags("", *kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig from path '%v', %v", *kubeConfigPath, err)
	}
	ocClient, err := client.NewForConfig(c)
	if err != nil {
		return nil, fmt.Errorf("error conveting rest client into OpenShift versioned client, %v", err)
	}
	return ocClient, nil
}

// GetInfrastructure gets information of current Infrastructure referred by the OpenShift client, each client should have only one infrastructure
// returns information of the infrastructure including InfrastructureName, cluster region
func GetInfrastructure(client *client.Clientset) (*v1.Infrastructure, error) {
	infra, err := client.ConfigV1().Infrastructures().List(metav1.ListOptions{})
	// we should only have 1 infrastructure
	if err != nil {
		return nil, fmt.Errorf("error getting infrastructure, %v", err)
	}
	if infra == nil || len(infra.Items) < 1 {
		return nil, fmt.Errorf("error getting infrastructure, no infrastructure present")
	}
	if len(infra.Items) > 1 {
		return nil, fmt.Errorf("error getting infrastructure, more than 1 infrastructure present. Existing number of infrastructures: %v", len(infra.Items))
	}
	return &infra.Items[0], nil
}
