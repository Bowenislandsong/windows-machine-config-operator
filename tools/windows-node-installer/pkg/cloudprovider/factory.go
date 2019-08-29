package cloudprovider

import (
	"fmt"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/cloudprovider/aws"
	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/openshiftcluster"
)

func NewCloudSessionFactory(kubeconfigPath, credentialPath, credAccount, dir *string) (Cloud, error) {
	// Initiate kubeclient and optain necessary information
	kubeclient, err := openshiftcluster.NewClient(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	infra, err := openshiftcluster.GetInfrastructure(kubeclient)
	if err != nil {
		return nil, err
	}

	// Select plat form based on cluster infrastructure information
	switch platform := infra.Status.PlatformStatus.Type; platform {
	case v1.AWSPlatformType:
		region := infra.Status.PlatformStatus.AWS.Region
		return aws.NewAws(kubeclient, credentialPath, credAccount, &region, dir)
	default:
		return nil, fmt.Errorf("failed to seletct cloud platform based on cluster infrastructure")
	}
}
