package rke

import (
	"os"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/hashicorp/hcl/v2/hclwrite"
	shepherdConfig "github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/framework/cleanup"
	"github.com/rancher/tfp-automation/framework/set/resources/rke/aws"
	rke "github.com/rancher/tfp-automation/framework/set/resources/rke/rke"
	resources "github.com/rancher/tfp-automation/framework/set/resources/sanity"
	"github.com/sirupsen/logrus"
)

const (
	kubeConfig             = "kube_config"
	rkeServerOnePublicIP   = "rke_server1_public_ip"
	rkeServerTwoPublicIP   = "rke_server2_public_ip"
	rkeServerThreePublicIP = "rke_server3_public_ip"
	terraformConst         = "terraform"
)

// CreateRKEMainTF is a helper function that will create the main.tf file for creating an RKE1 cluster
func CreateRKEMainTF(t *testing.T, terraformOptions *terraform.Options, keyPath string, rancherConfig *shepherdConfig.Config,
	terraformConfig *config.TerraformConfig, terratestConfig *config.TerratestConfig) (string, error) {
	var file *os.File
	file = resources.OpenFile(file, keyPath)
	defer file.Close()

	newFile := hclwrite.NewEmptyFile()
	rootBody := newFile.Body()

	tfBlock := rootBody.AppendNewBlock(terraformConst, nil)
	tfBlockBody := tfBlock.Body()

	logrus.Infof("Creating resources using AWS")
	file, err := aws.CreateAWSResources(file, newFile, tfBlockBody, rootBody, terraformConfig, terratestConfig)
	if err != nil {
		return "", err
	}

	_, err = terraform.InitAndApplyE(t, terraformOptions)
	if err != nil && *rancherConfig.Cleanup {
		logrus.Infof("Error while creating resources. Cleaning up...")
		cleanup.Cleanup(t, terraformOptions, keyPath)
		return "", err
	}

	rkeServerOnePublicIP := terraform.Output(t, terraformOptions, rkeServerOnePublicIP)

	file = resources.OpenFile(file, keyPath)
	logrus.Infof("Creating RKE cluster...")
	file, err = rke.CreateRKECluster(file, newFile, rootBody, terraformConfig)
	if err != nil {
		return "", err
	}

	err = appendKubeConfig(keyPath)
	if err != nil {
		return "", err
	}

	_, err = terraform.InitAndApplyE(t, terraformOptions)
	if err != nil && *rancherConfig.Cleanup {
		logrus.Infof("Error while creating RKE cluster. Cleaning up...")
		cleanup.Cleanup(t, terraformOptions, keyPath)
		return "", err
	}

	kubeConfigContent := terraform.Output(t, terraformOptions, kubeConfig)

	file = resources.OpenFile(file, keyPath)
	logrus.Infof("Checking RKE cluster status...")
	file, err = rke.CheckClusterStatus(file, newFile, rootBody, terraformConfig, terratestConfig, rkeServerOnePublicIP, kubeConfigContent)
	if err != nil {
		return "", err
	}

	_, err = terraform.InitAndApplyE(t, terraformOptions)
	if err != nil && *rancherConfig.Cleanup {
		logrus.Infof("Error while checking RKE status. Cleaning up...")
		cleanup.Cleanup(t, terraformOptions, keyPath)
		return "", err
	}

	err = removeKubeConfig(keyPath)
	if err != nil {
		return "", err
	}

	return rkeServerOnePublicIP, nil
}

// appendKubeConfig is a helper function that will append the kube_config output to the outputs.tf file
func appendKubeConfig(keyPath string) error {
	filePath := keyPath + "/outputs.tf"

	outputsFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	defer outputsFile.Close()

	outputBlock := `

output "kube_config" {
  value     = rke_cluster.cluster.kube_config_yaml
  sensitive = true
}
`
	if _, err := outputsFile.WriteString(outputBlock); err != nil {
		return err
	}

	return nil
}

// removeKubeConfig is a helper function that will remove the kube_config output block from the outputs.tf file
func removeKubeConfig(keyPath string) error {
	filePath := keyPath + "/outputs.tf"

	outputsFile, err := os.OpenFile(filePath, os.O_RDWR, 0600)
	if err != nil {
		return err
	}

	defer outputsFile.Close()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	outputBlock := `

output "kube_config" {
  value     = rke_cluster.cluster.kube_config_yaml
  sensitive = true
}
`
	contentStr := string(content)
	contentStr = strings.ReplaceAll(contentStr, outputBlock, "")

	err = os.WriteFile(filePath, []byte(contentStr), 0600)
	if err != nil {
		return err
	}

	return nil
}
