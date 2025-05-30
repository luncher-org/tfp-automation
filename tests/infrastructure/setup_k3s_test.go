package infrastructure

import (
	"os"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/hashicorp/hcl/v2/hclwrite"
	ranchFrame "github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/keypath"
	"github.com/rancher/tfp-automation/framework"
	"github.com/rancher/tfp-automation/framework/set/resources/k3s"
	"github.com/rancher/tfp-automation/framework/set/resources/providers"
	"github.com/rancher/tfp-automation/framework/set/resources/rancher2"
	"github.com/rancher/tfp-automation/framework/set/resources/sanity"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CreateK3SClusterTestSuite struct {
	suite.Suite
	terraformConfig  *config.TerraformConfig
	terratestConfig  *config.TerratestConfig
	terraformOptions *terraform.Options
}

const (
	k3sServerOne   = "k3s_server1"
	k3sServerTwo   = "k3s_server2"
	k3sServerThree = "k3s_server3"

	k3sServerOnePublicIP    = "k3s_server1_public_ip"
	k3sServerOnePrivateIP   = "k3s_server1_private_ip"
	k3sServerTwoPublicIP    = "k3s_server2_public_ip"
	k3sServerThreePublicIP  = "k3s_server3_public_ip"
	k3sBastionPublicIP      = "k3s_bastion_public_ip"
	k3sServerTwoPrivateIP   = "k3s_server2_private_ip"
	k3sServerThreePrivateIP = "k3s_server3_private_ip"
)

func (i *CreateK3SClusterTestSuite) TestCreateK3SCluster() {
	i.terraformConfig = new(config.TerraformConfig)
	ranchFrame.LoadConfig(config.TerraformConfigurationFileKey, i.terraformConfig)

	i.terratestConfig = new(config.TerratestConfig)
	ranchFrame.LoadConfig(config.TerratestConfigurationFileKey, i.terratestConfig)

	_, keyPath := rancher2.SetKeyPath(keypath.K3sKeyPath, i.terratestConfig.PathToRepo, i.terraformConfig.Provider)
	terraformOptions := framework.Setup(i.T(), i.terraformConfig, i.terratestConfig, keyPath)
	i.terraformOptions = terraformOptions

	var file *os.File
	file = sanity.OpenFile(file, keyPath)
	defer file.Close()

	newFile := hclwrite.NewEmptyFile()
	rootBody := newFile.Body()

	tfBlock := rootBody.AppendNewBlock(terraformConst, nil)
	tfBlockBody := tfBlock.Body()

	instances := []string{k3sServerOne, k3sServerTwo, k3sServerThree}

	providerTunnel := providers.TunnelToProvider(i.terraformConfig.Provider)
	file, err := providerTunnel.CreateNonAirgap(file, newFile, tfBlockBody, rootBody, i.terraformConfig, i.terratestConfig, instances)
	require.NoError(i.T(), err)

	terraform.InitAndApply(i.T(), terraformOptions)

	k3sServerOnePublicIP := terraform.Output(i.T(), terraformOptions, k3sServerOnePublicIP)
	k3sServerOnePrivateIP := terraform.Output(i.T(), terraformOptions, k3sServerOnePrivateIP)
	k3sServerTwoPublicIP := terraform.Output(i.T(), terraformOptions, k3sServerTwoPublicIP)
	k3sServerThreePublicIP := terraform.Output(i.T(), terraformOptions, k3sServerThreePublicIP)

	file = sanity.OpenFile(file, keyPath)
	logrus.Infof("Creating K3s cluster...")
	file, err = k3s.CreateK3SCluster(file, newFile, rootBody, i.terraformConfig, i.terratestConfig, k3sServerOnePublicIP, k3sServerOnePrivateIP, k3sServerTwoPublicIP, k3sServerThreePublicIP)
	require.NoError(i.T(), err)

	terraform.InitAndApply(i.T(), terraformOptions)

	logrus.Infof("Kubeconfig file is located in /home/%s/.kube in the node: %s", i.terraformConfig.Standalone.OSUser, k3sServerOnePublicIP)
}

func TestCreateK3SClusterTestSuite(t *testing.T) {
	suite.Run(t, new(CreateK3SClusterTestSuite))
}
