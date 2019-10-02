package e2e

import (
	"os"
	"testing"

	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/cloudprovider"
	"github.com/stretchr/testify/assert"
)

// TestE2ECreatingAndDestroyingWindowsInstanceOnEC2 creates and terminates Windows instance on AWS. After creation,
// it checks for the following properties on the instance:
// - instance exist
// - in running state
// - has public subnet
// - Public IP
// - OpenShift cluster Worker SG
// - Windows SG
// - OpenShift cluster IAM
// - has name
// - has OpenShift cluster vpc
// - instance and SG IDs are recorded to the json file
// After termination, it checks for the following:
// - instance terminated
// - Windows SG deleted
// - json file deleted
func TestE2ECreatingAndDestroyingWindowsInstanceOnEC2(t *testing.T) {
	// Get kubeconfig, AWS credentials, and artifact dir from environment variable set by the OpenShift CI operator.
	kubeconfig := os.Getenv("KUBECONFIG")
	awscredentials := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	dir := os.Getenv("ARTIFACT_DIR")

	awsCloud, err := cloudprovider.CloudProviderFactory(kubeconfig, awscredentials, "default", dir,
		"ami-0b8d82dea356226d3", "m4.large", "libra")
	assert.NoError(t, err, "error creating clients")

	// The e2e test assumes Microsoft Windows Server 2019 Base image, m4.large instance type, and libra sshkey are
	// available.
	err = awsCloud.CreateWindowsVM()
	assert.NoError(t, err, "error creating Windows instance")

	assert.NoError(t, err, "error reading from resource directory")

	err = awsCloud.DestroyWindowsVMs()
	assert.NoError(t, err, "error deleting instance")
}
