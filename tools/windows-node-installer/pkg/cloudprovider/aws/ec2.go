package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	sessionaws "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	v1 "github.com/openshift/api/config/v1"
	client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/cloudprovider"
	file "github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/fileutil"
	"github.com/openshift/windows-machine-config-operator/tools/windows-node-installer/pkg/openshiftcluster"
)

type InstancesInfo []InstanceInfo

type InstanceInfo struct {
	Instanceid string   `json:"Instanceid"`
	SG         []SGInfo `json:"SG"`
}

type SGInfo struct {
	Groupid   string `json:"Groupid"`
	Groupname string `json:"Groupname"`
}

// AWS specific information including:
// service account for EC2 and IAM
// clientset for OpenShift cluster
// dir path to where created instance and SG information is stored
//
// This struct should remain private since secretes should not be exposed
type awsSvc struct {
	svcEC2    *ec2.EC2
	svcIAM    *iam.IAM
	clientset *client.Clientset
	dir       *string
}

// Factory of AWS Cloud interface
func NewAws(clientset *client.Clientset, credentialPath, credAccount, region, dir *string) (cloudprovider.Cloud, error) {
	session, err := awsConfigSession(credentialPath, credAccount, region)
	if err != nil {
		return nil, err
	}
	return &awsSvc{ec2.New(session, aws.NewConfig()),
		iam.New(session, aws.NewConfig()),
		clientset,
		dir,
	}, nil
}

// AWSConfigSess uses AWS credentials to create a session for interacting with AWS EC2
// returns an AWS session
func awsConfigSession(credPath, credAccount, region *string) (*sessionaws.Session, error) {
	// Grab settings from flag values
	// TODO: Default values gear towards RedHat Boston Office (consider removing default values before public facing)
	if _, err := os.Stat(*credPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to find AWS credentials from path '%v'", *credPath)
	}
	sess := sessionaws.Must(sessionaws.NewSession(&aws.Config{
		Credentials: credentials.NewSharedCredentials(*credPath, *credAccount),
		Region:      aws.String(*region),
	}))
	return sess, nil
}

// CreateWindowsVM creates an Windows node instance based on a given AWS session and kubeconfig of an existing OpenShift cluster under the same VPC
// attaches existing OpenShift cluster worker security group and IAM
// uses public subnet, attaches public ip, and creates or attaches security group that allows all traffic 10.0.0.0/16 and RDP from my IP
// uses given image id, instance type, and keyname
// creates Name tag for the instance using the same prefix as the openshift cluster name
// writes id and security group information of an created instance
// provides RDP information in commandline
func (s *awsSvc) CreateWindowsVM(imageId, instanceType, keyName *string) error {
	var sgID, instanceID *string
	var createdInst InstanceInfo
	var createdSG SGInfo
	// get infrastructure from OC using kubeconfig info
	infra, err := openshiftcluster.GetInfrastructure(s.clientset)
	if err != nil {
		return err
	}
	// get infraID an unique readable id for the infrastructure
	infraID := &infra.Status.InfrastructureName
	// get vpc id of the openshift cluster
	vpcID, err := s.getVPCIdByInfrastructure(infra)
	if err != nil {
		return fmt.Errorf("failed to find cluster vpc, %v", err)
	}
	// get openshift cluster worker security groupID
	workerSG, err := s.getClusterWorkerSGID(infraID)
	if err != nil {

	}
	// get openshift cluster worker iam profile
	iamProfile, err := s.getIAMWorkerRole(infraID) // unnecessary, could just rely on naming convention to set the iam specifics
	// get or create a public subnet under the vpcID
	subnetID, err := s.getPubSubnetId(vpcID, infraID)

	// Specify the details of the instance
	runResult, err := s.svcEC2.RunInstances(&ec2.RunInstancesInput{
		ImageId:            imageId,
		InstanceType:       instanceType,
		KeyName:            keyName,
		SubnetId:           subnetID,
		MinCount:           aws.Int64(1),
		MaxCount:           aws.Int64(1),
		IamInstanceProfile: iamProfile,
		SecurityGroupIds:   []*string{sgID, workerSG},
	})
	if err != nil {
		log.Fatalf("Could not create instance: %v", err)
	} else {
		instanceID = runResult.Instances[0].InstanceId
		createdInst = InstanceInfo{
			Instanceid: *instanceID,
			SG:         []SGInfo{createdSG},
		}
		log.Println("Created instance", *instanceID)
	}
	_, err = s.svcEC2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: sgID,
		IpPermissions: []*ec2.IpPermission{
			(&ec2.IpPermission{}).
				SetIpProtocol("-1").
				SetIpRanges([]*ec2.IpRange{
					(&ec2.IpRange{}).
						SetCidrIp("10.0.0.0/16"),
				}),
			(&ec2.IpPermission{}).
				SetIpProtocol("tcp").
				SetFromPort(3389).
				SetToPort(3389).
				SetIpRanges([]*ec2.IpRange{
					(&ec2.IpRange{}).
						SetCidrIp(getMyIp() + "/32"),
				}),
		},
	})
	if err != nil {
		log.Printf("unable to set security group ingress, %v", err)
	}
	// Add tags to the created instance
	_, err = s.svcEC2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{runResult.Instances[0].InstanceId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(*infraID + "-winNode"),
			},
		},
	})
	if err != nil {
		log.Println("Could not create tags for instance", runResult.Instances[0].InstanceId, err)
	}
	ipRes, err := s.allocatePublicIp()
	if err != nil {
		log.Printf("error allocating public ip to associate with instance, please manually allocate public ip, %v", err)
	} else {
		log.Println("waiting for the vm to be ready for attaching a public ip address...")
		err = s.svcEC2.WaitUntilInstanceStatusOk(&ec2.DescribeInstanceStatusInput{
			InstanceIds: []*string{instanceID},
		})
		if err != nil {
			log.Printf("failed to wait for instance to be ok, %v", err)
		}
		_, err = s.svcEC2.AssociateAddress(&ec2.AssociateAddressInput{
			AllocationId: ipRes.AllocationId,
			InstanceId:   instanceID,
		})
		if err != nil {
			log.Printf("failed to associate public ip for instance, %v", err)
		}
	}
	err = writeAWSInstanceInfo(&Instances{createdInst}, s.dir)
	if err != nil {
		log.Panicf("failed to write instance info to file at '%v', instance will not be able to be deleted, %v", *s.dir, err)
	}
	log.Println("Successfully created windows node instance, please RDP into windows with the following:")
	log.Printf("xfreerdp /u:Administrator /v:%v  /h:1080 /w:1920 /p:'Secret2018'", *ipRes.PublicIp)
	return nil
}

// DestroyWindowsVM destroys instances and security groups on AWS specified in file from the path and consume the file if succeeded
func (s *awsSvc) DestroyWindowsVM() error {
	instances, err := readAWSInstanceInfo(s.dir)
	log.Printf("consuming file '%v'", *s.dir)
	if err != nil {
		return fmt.Errorf("failed to read file from '%v', instance not deleted, %v", *s.dir, err)
	}
	for _, inst := range *instances {
		for _, sg := range inst.SG {
			if sg.Groupid == "" {
				continue
			}
			err = s.deleteSG(sg.Groupid)
			if err != nil {
				log.Printf("failed to delete security group: %v, %v", sg.Groupname, err)
			}
		}
		err = s.deleteInstance(inst.Instanceid)
		if err != nil {
			log.Printf("failed to delete instance '%v', %v", inst.Instanceid, err)
		}
	}
	if err == nil {
		err = os.Remove(*s.dir)
		if err != nil {
			log.Printf("failed to delete file at '%v'", err)
		}
	} else {
		log.Printf("file '%v' not deleted due to deletion error, %v", *s.dir, err)
	}
	return nil
}

// deleteSG will delete security group based on group id
// return error if deletion fails
func (s *awsSvc) deleteSG(groupid string) error {
	_, err := s.svcEC2.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(groupid),
	})
	return err
}

func (s *awsSvc) searchOrCreateSg(infraID, vpcID *string) (*SGInfo, error) {
	var sgID *string
	sg, err := s.svcEC2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(*infraID + "-winc-sg"),
		Description: aws.String("security group for rdp and all traffic"),
		VpcId:       vpcID,
	})
	if err != nil {
		log.Printf("could not create Security Group, attaching existing instead: %v", err)
		sgs, err := s.svcEC2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("vpc-id"),
					Values: []*string{vpcID},
				},
				{
					Name:   aws.String("group-name"),
					Values: aws.StringSlice([]string{*infraID + "-winc-sg"}),
				},
			},
		})
		if err != nil || sgs == nil || len(sgs.SecurityGroups) == 0 {
			return nil, fmt.Errorf("failed to create or find security group, %v", err)
		}
		sgID = sgs.SecurityGroups[0].GroupId
	} else {
		sgID = sg.GroupId
		// we only delete security group that is created with the instance.
		// If it is reused, we will not log or delete SG when removing instances that are borrowing the SG.
	}
	return &SGInfo{
		Groupname: *infraID + "-winc-sg",
		Groupid:   *sgID,
	}, nil
}

// deleteInstance will delete an AWS instance based on instance id
// return error if deletion fails
func (s *awsSvc) deleteInstance(instanceID string) error {
	_, err := s.svcEC2.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	return err
}

// writeInstanceInfo logs details of an created instance and append to a json file about instance id and attached security group
// return error if file write fails
func writeAWSInstanceInfo(info *Instances, path *string) error {
	pastinfo, err := readAWSInstanceInfo(path)
	if err == nil {
		for _, past := range *pastinfo {
			*info = append(*info, past)
		}
	}
	newinfo, err := json.Marshal(*info)
	if err != nil {
		return fmt.Errorf("failed to marshal information into json format, %v", err)
	}
	err = ioutil.WriteFile(*path, newinfo, 0644)
	if err != nil {
		return fmt.Errorf("failed to write instance info to file, deletion will have to be manual, %v", err)
	}
	return nil
}

// readInstanceInfo reads from a json file about instance id and attached security group
// return instance information including instance id, instance attached security group id and name, and error if fails
func readAWSInstanceInfo(path *string) (Instances, error) {
	return file.ReadInstanceInfo(Instances, path)
}

// getMyIp get the external IP of current machine from http://myexternalip.com
// TODO: Find a more reliable strategy than relying on a website
// returns external IP
func getMyIp() string {
	resp, err := http.Get("http://myexternalip.com/raw")
	if err != nil {
		log.Panic("Failed to get external IP Addr")
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		log.Panic("Failed to read external IP Addr")
	}
	return buf.String()
}

// allocatePublicIp find a randomly assigned ip by AWS that is available
// returns public ip related information and error messages if any
func (s *awsSvc) allocatePublicIp() (*ec2.AllocateAddressOutput, error) {
	ip, err := s.svcEC2.AllocateAddress(&ec2.AllocateAddressInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to allocate an elastic ip, please assign public ip manually, %v", err)
	}
	return ip, nil
}

// getVPCByInfrastructure gets VPC of the infrastructure.
// returns VPC id and error messages
func (s *awsSvc) getVPCIdByInfrastructure(infra *v1.Infrastructure) (*string, error) {
	res, err := s.svcEC2.DescribeVpcs(&ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: aws.StringSlice([]string{infra.Status.InfrastructureName + "-vpc"}), //TODO: use this kubernetes.io/cluster/{infraName}: owned
			},
			{
				Name:   aws.String("state"),
				Values: aws.StringSlice([]string{"available"}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Unable to describe VPCs, %v", err)
	}
	if len(res.Vpcs) == 0 {
		return nil, fmt.Errorf("no VPCs found")
	} else if len(res.Vpcs) > 1 {
		//TODO: whatelse can i do than panic about an error but dont want to return
		log.Panicf("More than one VPCs are found, we returned the first one")
	}
	//vpcAttri, err := svc.DescribeVpcAttribute(&ec2.DescribeVpcAttributeInput{
	//	Attribute:aws.String(ec2.VpcAttributeNameEnableDnsSupport),
	//	VpcId: res.Vpcs[0].VpcId,
	//})
	//if err != nil {
	//	log.Printf("failed to find vpc attribute, no public DNS assigned, %v", err)
	//}
	//vpcAttri.SetEnableDnsHostnames(&ec2.AttributeBooleanValue{Value: aws.Bool(true)})
	//vpcAttri.SetEnableDnsSupport(&ec2.AttributeBooleanValue{Value: aws.Bool(true)})
	return res.Vpcs[0].VpcId, err
}

//func isVpcEnabledDns()

// getPubSubnetOrCreate gets the public subnet under a given vpc id. If no subnet is available, then it creates one.
// returns subent id and error messages
func (s *awsSvc) getPubSubnetId(vpcID, infraID *string) (*string, error) {
	// search subnet by the vpcid owned by the vpcID
	subnets, err := s.svcEC2.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{vpcID},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find a public subnet based on the given VpcID: %v, %v", vpcID, err)
	}
	for _, subnet := range subnets.Subnets { // find public subnet within the vpc
		for _, tag := range subnet.Tags {
			// TODO: find public subnet by checking something like routing privileges.
			if *tag.Key == "Name" && strings.Contains(*tag.Value, *infraID+"-public-") {
				return subnet.SubnetId, err
			}
		}
	}
	return nil, fmt.Errorf("failed to find public subnet in vpc: %v", vpcID)
}

// getClusterSGID gets worker security group id from an existing cluster
// returns security group id
func (s *awsSvc) getClusterWorkerSGID(infraID *string) (*string, error) {
	sg, err := s.svcEC2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: aws.StringSlice([]string{*infraID + "-worker-sg"}),
			},
			{
				Name:   aws.String("tag:kubernetes.io/cluster/" + *infraID),
				Values: aws.StringSlice([]string{"owned"}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach security group of openshift cluster worker, please manually add it, %v", err)
	}
	if sg == nil || len(sg.SecurityGroups) < 1 {
		return nil, fmt.Errorf("no security groups is found for the cluster worker nodes, please add cluster worker SG manually")
	}
	if len(sg.SecurityGroups) > 1 {
		return nil, fmt.Errorf("more than one security groups are found for the cluster worker nodes, please add cluster worker SG manually")
	}
	return sg.SecurityGroups[0].GroupId, nil
}

// getIAMrole gets IAM information from an existing cluster either 'worker' or 'master'
// returns IAM information including IAM arn, and nonfatal error
func (s *awsSvc) getIAMWorkerRole(infraID *string) (*ec2.IamInstanceProfileSpecification, error) {
	iamspc, err := s.svcIAM.GetInstanceProfile(&iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(fmt.Sprintf("%s-worker-profile", *infraID)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find iam role, please attache manually %v", err)
	}
	return &ec2.IamInstanceProfileSpecification{
		Arn: iamspc.InstanceProfile.Arn,
	}, nil
}
