# windows-node-installer
The `windows-node-installer` is a tool that creates a Windows instance under the same virtual network (AWS-VCP, Azure-Vnet) used by a given OpenShift cluster running on select platforms.
The tool configures the instance to allow communication with the worker nodes of the running cluster.

## Supported Platforms
 - AWS EC2

## pre-requisite
- An existing OpenShift cluster running on a platform.
- AWS EC2 credentials (aws_access_key_id and aws_access_key_id)
- kubeconfig of the existing OpenShift Cluster

## What it does
The `windows-node-installer` tool **creates** a Windows node (Windows server 2019) under the same virtual network as OpenShift Cluster.
The tool configures the Windows node with the following properties:
- Node Name \<OpenShift Cluster Name\>-winNode
- A m4.large instance
- Shared vpc with OpenShift Cluster
- Public Subnet (within the vpc)
- Auto-assign Public IP
- Using shared libra key
- security group (secure public IP RDP with my IP and 10.x/16)
- Attach IAM role (OpenShift Cluster Worker Profile)
- Attach Security Group (OpenShift Cluster - Worker)

At last, the tool outputs a way to RDP inside of the Windows node.

The `windows-node-installer` tool also provides a way to **destroy** the Windows node it created and delete its security group.

## Getting Started
Install:
```bash
git clone https://github.com/openshift/windows-machine-config-operator.git
cd windows-machine-config-operator/tools/winc-winc-setup
export GO111MODULE=on
go build .
```
Create a windows node:
```bash
./winc-vm-setup create -awscred=/abs/path/to/your/aws/credentials -kubeconfig=/abs/path/to/your/kubeconfig
```
Destroy the windows nodes created:
```bash
./winc-winc-setup destroy -awscred=/abs/path/to/your/aws/credentials
```
## Usage
Creating a Windows Node:
```bash
./winc-winc-setup create -h
Usage of create:
  --aws-account string
    	account name of the aws credentials (default "openshift-dev")
  --aws-cred string
    	Required: absolute path of aws credentials
  -dir string
    	path to 'winc-setup.json'. (default "./")
  -imageid string
    	Set instance AMI ID tobe deployed. AWS windows server 2019 is ami-04ca2d0801450d495. (default "ami-0943eb2c39917fc11")
  -instancetype string
    	Set instance type tobe deployed. Free tier is t2.micro. (default "m4.large")
  -keyname string
    	Set key.pem for accessing the instance. (default "libra")
  -kubeconfig string
    	Required: absolute path to the kubeconfig file
  -region string
    	Set region where the instance will be running on aws (default "us-east-1")
```
Destroying Windows Nodes:
```bash
./winc-winc-setup destroy -h
Usage of destroy:
  -awsaccount string
    	account name of the aws credentials (default "openshift-dev")
  -awscred string
    	Required: absolute path of aws credentials
  -dir string
    	path to 'winc-setup.json'. (default "./")
  -region string
    	Set region where the instance will be running on aws (default "us-east-1")
```
## Future Work 
1. Ansible
    - firewall
    - powershell
