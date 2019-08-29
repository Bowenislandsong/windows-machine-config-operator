package main

import (
	"flag"
	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/cloudprovider"
	"log"
	"os"
	
	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/cloudprovider/aws"
	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/openshiftcluster"
)

const (
	installerFileName = "windows-node-installer.json"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please use one argument either 'create' or 'destroy' a node")
	}
	createFlag := flag.NewFlagSet("create", flag.ExitOnError)
	destroyFlag := flag.NewFlagSet("destroy", flag.ExitOnError)
	if os.Args[1] == "create" {
		// subflags of create
		// openshift cluster region
		credPath := createFlag.String("awscred", "", "Required: absolute path of aws credentials")
		credAccount := createFlag.String("awsaccount", "openshift-dev", "account name of the aws credentials") // Default accounts for dev team in OpenShift
		dir := createFlag.String("dir", "./", "path to 'winc-setup.json'.")
		region := createFlag.String("region", "us-east-1", "Set region where the instance will be running on aws") // Default region for Boston Office or East Coast (virginia)
		// existing kubeconfig for the openshift cluster (one per cluster)
		kubeConfigPath := createFlag.String("kubeconfig", "", "Required: absolute path to the kubeconfig file")
		// Set image by AMI ID. Default using Aravindh's firewall disabled image ami-0943eb2c39917fc11 (Does not always have firewall disabled) AWS windows server 2019 is ami-04ca2d0801450d495
		imageId := createFlag.String("imageid", "ami-0943eb2c39917fc11", "Set instance AMI ID tobe deployed. AWS windows server 2019 is ami-04ca2d0801450d495.")
		// set instance type. Free tier is t2.micro
		instanceType := createFlag.String("instancetype", "m4.large", "Set instance type tobe deployed. Free tier is t2.micro.")
		// set key name for authentication
		keyName := createFlag.String("keyname", "libra", "Set key.pem for accessing the instance.")
		// parse flags
		err := createFlag.Parse(os.Args[2:])
		if err != nil {
			println("Please get help with 'create -h'.")
		}

		// Create node instance on selected platform
		cloudprovider.NewCloudSessionFactory()
		if err != nil {
			log.Fatalf("Failed to get client, %v", err)
		}
		*dir = *dir + "winc-setup.json"
		aws.CreateWindowsVM(sessAWS, oc, imageId, instanceType, keyName, dir)
	} else if os.Args[1] == "destroy" {
		// subflags of destroy
		credPath := destroyFlag.String("awscred", "", "Required: absolute path of aws credentials")
		credAccount := destroyFlag.String("awsaccount", "openshift-dev", "account name of the aws credentials")     // Default accounts for dev team in OpenShift
		region := destroyFlag.String("region", "us-east-1", "Set region where the instance will be running on aws") // Default region for Boston Office or East Coast (virginia)
		dir := destroyFlag.String("dir", "./", "path to 'winc-setup.json'.")
		// parse flags
		err := destroyFlag.Parse(os.Args[2:])
		if err != nil {
			println("Please get help with 'destroy -h'.")
		}
		sessAWS := openshiftcluster.AWSConfigSess(credPath, credAccount, region)
		*dir = *dir + "winc-setup.json"
		aws.DestroyWindowsVM(sessAWS, dir)
	} else {
		log.Fatal("Please use one argument either 'create' or 'destroy' a node")
	}

}
