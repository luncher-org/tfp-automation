package rbac

import (
	"os"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/shepherd/clients/rancher"
	shepherdConfig "github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/config/operations"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/authproviders"
	"github.com/rancher/tfp-automation/defaults/configs"
	"github.com/rancher/tfp-automation/defaults/keypath"
	"github.com/rancher/tfp-automation/framework"
	"github.com/rancher/tfp-automation/framework/cleanup"
	"github.com/rancher/tfp-automation/framework/set/resources/rancher2"
	qase "github.com/rancher/tfp-automation/pipeline/qase/results"
	"github.com/rancher/tfp-automation/tests/extensions/provisioning"
	rb "github.com/rancher/tfp-automation/tests/extensions/rbac"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AuthConfigTestSuite struct {
	suite.Suite
	client           *rancher.Client
	session          *session.Session
	cattleConfig     map[string]any
	rancherConfig    *rancher.Config
	terraformConfig  *config.TerraformConfig
	terratestConfig  *config.TerratestConfig
	terraformOptions *terraform.Options
}

func (r *AuthConfigTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.cattleConfig = shepherdConfig.LoadConfigFromFile(os.Getenv(shepherdConfig.ConfigEnvironmentKey))
	configMap, err := provisioning.UniquifyTerraform([]map[string]any{r.cattleConfig})
	require.NoError(r.T(), err)

	r.cattleConfig = configMap[0]
	r.rancherConfig, r.terraformConfig, r.terratestConfig = config.LoadTFPConfigs(r.cattleConfig)

	keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath, "")
	terraformOptions := framework.Setup(r.T(), r.terraformConfig, r.terratestConfig, keyPath)
	r.terraformOptions = terraformOptions
}

func (r *AuthConfigTestSuite) TestTfpAuthConfig() {
	tests := []struct {
		name         string
		authProvider string
	}{
		{"Azure AD", authproviders.AzureAD},
		{"GitHub", authproviders.GitHub},
		{"Okta", authproviders.Okta},
		{"OpenLDAP", authproviders.OpenLDAP},
	}

	for _, tt := range tests {
		configMap := []map[string]any{r.cattleConfig}

		operations.ReplaceValue([]string{"terraform", "authProvider"}, tt.authProvider, configMap[0])

		_, terraform, _ := config.LoadTFPConfigs(configMap[0])

		testUser, testPassword := configs.CreateTestCredentials()

		r.Run((tt.name), func() {
			keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath, "")
			defer cleanup.Cleanup(r.T(), r.terraformOptions, keyPath)

			rb.AuthConfig(r.T(), terraform, r.terraformOptions, testUser, testPassword, configMap)
		})
	}

	if r.terratestConfig.LocalQaseReporting {
		qase.ReportTest()
	}
}

func (r *AuthConfigTestSuite) TestTfpAuthConfigDynamicInput() {
	if r.terraformConfig.AuthProvider == "" {
		r.T().Skip("No auth provider specified")
	}

	tests := []struct {
		name string
	}{
		{r.terraformConfig.AuthProvider},
	}

	for _, tt := range tests {
		configMap := []map[string]any{r.cattleConfig}

		operations.ReplaceValue([]string{"terraform", "authProvider"}, r.terraformConfig.AuthProvider, configMap[0])

		testUser, testPassword := configs.CreateTestCredentials()

		r.Run((tt.name), func() {
			keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath, "")
			defer cleanup.Cleanup(r.T(), r.terraformOptions, keyPath)

			rb.AuthConfig(r.T(), r.terraformConfig, r.terraformOptions, testUser, testPassword, configMap)
		})
	}

	if r.terratestConfig.LocalQaseReporting {
		qase.ReportTest()
	}
}

func TestTfpAuthConfigTestSuite(t *testing.T) {
	suite.Run(t, new(AuthConfigTestSuite))
}
