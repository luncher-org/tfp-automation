package rke

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/keypath"
	"github.com/rancher/tfp-automation/framework"
	"github.com/rancher/tfp-automation/framework/cleanup"
	"github.com/rancher/tfp-automation/framework/set/resources/rancher2"
	rke "github.com/rancher/tfp-automation/framework/set/resources/rke"
	qase "github.com/rancher/tfp-automation/pipeline/qase/results"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKEProviderTestSuite struct {
	suite.Suite
	client           *rancher.Client
	session          *session.Session
	cattleConfig     map[string]any
	rancherConfig    *rancher.Config
	terraformConfig  *config.TerraformConfig
	terratestConfig  *config.TerratestConfig
	terraformOptions *terraform.Options
}

func (t *RKEProviderTestSuite) TearDownSuite() {
	_, keyPath := rancher2.SetKeyPath(keypath.RKEKeyPath, t.terratestConfig.PathToRepo, t.terraformConfig.Provider)
	cleanup.Cleanup(t.T(), t.terraformOptions, keyPath)
}

func (t *RKEProviderTestSuite) TestCreateRKECluster() {
	t.rancherConfig, t.terraformConfig, t.terratestConfig = config.LoadTFPConfigs(t.cattleConfig)

	_, keyPath := rancher2.SetKeyPath(keypath.RKEKeyPath, t.terratestConfig.PathToRepo, t.terraformConfig.Provider)
	terraformOptions := framework.Setup(t.T(), t.terraformConfig, t.terratestConfig, keyPath)
	t.terraformOptions = terraformOptions

	_, err := rke.CreateRKEMainTF(t.T(), t.terraformOptions, keyPath, t.rancherConfig, t.terraformConfig, t.terratestConfig)
	require.NoError(t.T(), err)

	if t.terratestConfig.LocalQaseReporting {
		qase.ReportTest(t.terratestConfig)
	}
}

func TestRKEProviderTestSuite(t *testing.T) {
	suite.Run(t, new(RKEProviderTestSuite))
}
