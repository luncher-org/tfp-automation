package recurring

import (
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/config/operations"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/tests/actions/qase"
	"github.com/rancher/tests/validation/provisioning/resources/standarduser"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/clustertypes"
	"github.com/rancher/tfp-automation/defaults/configs"
	"github.com/rancher/tfp-automation/defaults/keypath"
	"github.com/rancher/tfp-automation/defaults/modules"
	"github.com/rancher/tfp-automation/framework/cleanup"
	"github.com/rancher/tfp-automation/framework/set/provisioning/imported"
	"github.com/rancher/tfp-automation/framework/set/resources/rancher2"
	tfpQase "github.com/rancher/tfp-automation/pipeline/qase"
	"github.com/rancher/tfp-automation/pipeline/qase/results"
	"github.com/rancher/tfp-automation/tests/extensions/provisioning"
	"github.com/rancher/tfp-automation/tests/infrastructure"
	"github.com/rancher/tfp-automation/tests/rancher2/snapshot"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TfpRancher2RecurringRunsTestSuite struct {
	suite.Suite
	client                     *rancher.Client
	standardUserClient         *rancher.Client
	session                    *session.Session
	cattleConfig               map[string]any
	rancherConfig              *rancher.Config
	terraformConfig            *config.TerraformConfig
	terratestConfig            *config.TerratestConfig
	standaloneConfig           *config.Standalone
	standaloneTerraformOptions *terraform.Options
	terraformOptions           *terraform.Options
}

func (r *TfpRancher2RecurringRunsTestSuite) TearDownSuite() {
	_, keyPath := rancher2.SetKeyPath(keypath.SanityKeyPath, r.terratestConfig.PathToRepo, r.terraformConfig.Provider)
	cleanup.Cleanup(r.T(), r.standaloneTerraformOptions, keyPath)
}

func (r *TfpRancher2RecurringRunsTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.client, _, r.standaloneTerraformOptions, r.terraformOptions, r.cattleConfig = infrastructure.SetupRancher(r.T(), r.session, keypath.SanityKeyPath)
	r.rancherConfig, r.terraformConfig, r.terratestConfig, r.standaloneConfig = config.LoadTFPConfigs(r.cattleConfig)
}

func (r *TfpRancher2RecurringRunsTestSuite) TestTfpRecurringProvisionCustomCluster() {
	var err error
	var testUser, testPassword string

	customClusterNames := []string{}

	r.standardUserClient, testUser, testPassword, err = standarduser.CreateStandardUser(r.client)
	require.NoError(r.T(), err)

	standardUserToken, err := infrastructure.CreateStandardUserToken(r.T(), r.terraformOptions, r.rancherConfig, testUser, testPassword)
	require.NoError(r.T(), err)

	standardToken := standardUserToken.Token

	tests := []struct {
		name   string
		module string
	}{
		{"Custom_TFP_RKE2", modules.CustomEC2RKE2},
		{"Custom_TFP_RKE2_Windows_2019", modules.CustomEC2RKE2Windows2019},
		{"Custom_TFP_RKE2_Windows_2022", modules.CustomEC2RKE2Windows2022},
		{"Custom_TFP_K3S", modules.CustomEC2K3s},
	}

	for _, tt := range tests {
		newFile, rootBody, file := rancher2.InitializeMainTF(r.terratestConfig)
		defer file.Close()

		configMap, err := provisioning.UniquifyTerraform([]map[string]any{r.cattleConfig})
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"rancher", "adminToken"}, standardToken, configMap[0])
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"terraform", "module"}, tt.module, configMap[0])
		require.NoError(r.T(), err)

		provisioning.GetK8sVersion(r.T(), r.standardUserClient, r.terratestConfig, r.terraformConfig, configs.DefaultK8sVersion, configMap)

		rancher, terraform, terratest, _ := config.LoadTFPConfigs(configMap[0])

		r.Run((tt.name), func() {
			_, keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath, r.terratestConfig.PathToRepo, "")
			defer cleanup.Cleanup(r.T(), r.terraformOptions, keyPath)

			clusterIDs, customClusterNames := provisioning.Provision(r.T(), r.client, r.standardUserClient, rancher, terraform, terratest, testUser, testPassword, r.terraformOptions, configMap, newFile, rootBody, file, false, false, true, customClusterNames)
			provisioning.VerifyClustersState(r.T(), r.client, clusterIDs)

			if strings.Contains(terraform.Module, clustertypes.WINDOWS) {
				clusterIDs, _ = provisioning.Provision(r.T(), r.client, r.standardUserClient, rancher, terraform, terratest, testUser, testPassword, r.terraformOptions, configMap, newFile, rootBody, file, true, true, true, customClusterNames)
				provisioning.VerifyClustersState(r.T(), r.client, clusterIDs)
			}
		})

		params := tfpQase.GetProvisioningSchemaParams(configMap[0])
		err = qase.UpdateSchemaParameters(tt.name, params)
		if err != nil {
			logrus.Warningf("Failed to upload schema parameters %s", err)
		}
	}

	if r.terratestConfig.LocalQaseReporting {
		results.ReportTest(r.terratestConfig)
	}
}

func (r *TfpRancher2RecurringRunsTestSuite) TestTfpRecurringProvisionImportedCluster() {
	var err error
	var testUser, testPassword string

	r.standardUserClient, testUser, testPassword, err = standarduser.CreateStandardUser(r.client)
	require.NoError(r.T(), err)

	standardUserToken, err := infrastructure.CreateStandardUserToken(r.T(), r.terraformOptions, r.rancherConfig, testUser, testPassword)
	require.NoError(r.T(), err)

	standardToken := standardUserToken.Token

	tests := []struct {
		name   string
		module string
	}{
		{"Upgrade_Imported_RKE2", modules.ImportEC2RKE2},
		{"Upgrade_Imported_RKE2_Windows_2019", modules.ImportEC2RKE2Windows2019},
		{"Upgrade_Imported_RKE2_Windows_2022", modules.ImportEC2RKE2Windows2022},
		{"Upgrade_Imported_K3S", modules.ImportEC2K3s},
	}

	for _, tt := range tests {
		newFile, rootBody, file := rancher2.InitializeMainTF(r.terratestConfig)
		defer file.Close()

		configMap, err := provisioning.UniquifyTerraform([]map[string]any{r.cattleConfig})
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"rancher", "adminToken"}, standardToken, configMap[0])
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"terraform", "module"}, tt.module, configMap[0])
		require.NoError(r.T(), err)

		rancher, terraform, terratest, _ := config.LoadTFPConfigs(configMap[0])

		r.Run((tt.name), func() {
			_, keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath, r.terratestConfig.PathToRepo, "")
			defer cleanup.Cleanup(r.T(), r.terraformOptions, keyPath)

			clusterIDs, _ := provisioning.Provision(r.T(), r.client, r.standardUserClient, rancher, terraform, terratest, testUser, testPassword, r.terraformOptions, configMap, newFile, rootBody, file, false, false, true, nil)
			provisioning.VerifyClustersState(r.T(), r.client, clusterIDs)

			err = imported.SetUpgradeImportedCluster(r.client, terraform)
			require.NoError(r.T(), err)
		})

		params := tfpQase.GetProvisioningSchemaParams(configMap[0])
		err = qase.UpdateSchemaParameters(tt.name, params)
		if err != nil {
			logrus.Warningf("Failed to upload schema parameters %s", err)
		}
	}

	if r.terratestConfig.LocalQaseReporting {
		results.ReportTest(r.terratestConfig)
	}
}

func (r *TfpRancher2RecurringRunsTestSuite) TestTfpRecurringPSACT() {
	if strings.Contains(r.terraformConfig.Standalone.RancherTagVersion, "v2.11") {
		r.T().Skip("Rancher Baseline has a known issue with Rancher versions 2.11 and below. Skipping PSACT tests.")
	}

	var err error
	var testUser, testPassword string

	r.standardUserClient, testUser, testPassword, err = standarduser.CreateStandardUser(r.client)
	require.NoError(r.T(), err)

	standardUserToken, err := infrastructure.CreateStandardUserToken(r.T(), r.terraformOptions, r.rancherConfig, testUser, testPassword)
	require.NoError(r.T(), err)

	standardToken := standardUserToken.Token

	nodeRolesDedicated := []config.Nodepool{config.EtcdNodePool, config.ControlPlaneNodePool, config.WorkerNodePool}

	tests := []struct {
		name      string
		module    string
		nodeRoles []config.Nodepool
		psact     config.PSACT
	}{
		{"RKE2_Rancher_Privileged", modules.EC2RKE2, nodeRolesDedicated, "rancher-privileged"},
		{"RKE2_Rancher_Restricted", modules.EC2RKE2, nodeRolesDedicated, "rancher-restricted"},
		{"RKE2_Rancher_Baseline", modules.EC2RKE2, nodeRolesDedicated, "rancher-baseline"},
		{"K3S_Rancher_Privileged", modules.EC2K3s, nodeRolesDedicated, "rancher-privileged"},
		{"K3S_Rancher_Restricted", modules.EC2K3s, nodeRolesDedicated, "rancher-restricted"},
		{"K3S_Rancher_Baseline", modules.EC2K3s, nodeRolesDedicated, "rancher-baseline"},
	}

	for _, tt := range tests {
		newFile, rootBody, file := rancher2.InitializeMainTF(r.terratestConfig)
		defer file.Close()

		configMap, err := provisioning.UniquifyTerraform([]map[string]any{r.cattleConfig})
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"rancher", "adminToken"}, standardToken, configMap[0])
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"terraform", "module"}, tt.module, configMap[0])
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"terratest", "nodepools"}, tt.nodeRoles, configMap[0])
		require.NoError(r.T(), err)

		rancher, terraform, terratest, _ := config.LoadTFPConfigs(configMap[0])

		r.Run((tt.name), func() {
			_, keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath, r.terratestConfig.PathToRepo, "")
			defer cleanup.Cleanup(r.T(), r.terraformOptions, keyPath)

			clusterIDs, _ := provisioning.Provision(r.T(), r.client, r.standardUserClient, rancher, terraform, terratest, testUser, testPassword, r.terraformOptions, configMap, newFile, rootBody, file, false, false, true, nil)
			provisioning.VerifyClustersState(r.T(), r.client, clusterIDs)
			provisioning.VerifyClusterPSACT(r.T(), r.client, clusterIDs)
		})

		params := tfpQase.GetProvisioningSchemaParams(configMap[0])
		err = qase.UpdateSchemaParameters(tt.name, params)
		if err != nil {
			logrus.Warningf("Failed to upload schema parameters %s", err)
		}
	}

	if r.terratestConfig.LocalQaseReporting {
		results.ReportTest(r.terratestConfig)
	}
}

func (r *TfpRancher2RecurringRunsTestSuite) TestTfpRecurringSnapshotRestore() {
	var err error
	var testUser, testPassword string

	r.standardUserClient, testUser, testPassword, err = standarduser.CreateStandardUser(r.client)
	require.NoError(r.T(), err)

	standardUserToken, err := infrastructure.CreateStandardUserToken(r.T(), r.terraformOptions, r.rancherConfig, testUser, testPassword)
	require.NoError(r.T(), err)

	standardToken := standardUserToken.Token

	nodeRolesDedicated := []config.Nodepool{config.EtcdNodePool, config.ControlPlaneNodePool, config.WorkerNodePool}

	snapshotRestoreNone := config.TerratestConfig{
		SnapshotInput: config.Snapshots{
			SnapshotRestore: "none",
		},
	}

	tests := []struct {
		name         string
		module       string
		nodeRoles    []config.Nodepool
		etcdSnapshot config.TerratestConfig
	}{
		{"RKE2_Snapshot_Restore", modules.EC2RKE2, nodeRolesDedicated, snapshotRestoreNone},
		{"K3S_Snapshot_Restore", modules.EC2K3s, nodeRolesDedicated, snapshotRestoreNone},
	}

	for _, tt := range tests {
		newFile, rootBody, file := rancher2.InitializeMainTF(r.terratestConfig)
		defer file.Close()

		configMap, err := provisioning.UniquifyTerraform([]map[string]any{r.cattleConfig})
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"rancher", "adminToken"}, standardToken, configMap[0])
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"terraform", "module"}, tt.module, configMap[0])
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"terratest", "nodepools"}, tt.nodeRoles, configMap[0])
		require.NoError(r.T(), err)

		_, err = operations.ReplaceValue([]string{"terratest", "snapshotInput", "snapshotRestore"}, tt.etcdSnapshot.SnapshotInput.SnapshotRestore, configMap[0])
		require.NoError(r.T(), err)

		provisioning.GetK8sVersion(r.T(), r.standardUserClient, r.terratestConfig, r.terraformConfig, configs.DefaultK8sVersion, configMap)

		rancher, terraform, terratest, _ := config.LoadTFPConfigs(configMap[0])

		r.Run(tt.name, func() {
			_, keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath, r.terratestConfig.PathToRepo, "")
			defer cleanup.Cleanup(r.T(), r.terraformOptions, keyPath)

			clusterIDs, _ := provisioning.Provision(r.T(), r.client, r.standardUserClient, rancher, terraform, terratest, testUser, testPassword, r.terraformOptions, configMap, newFile, rootBody, file, false, false, false, nil)
			provisioning.VerifyClustersState(r.T(), r.client, clusterIDs)

			snapshot.RestoreSnapshot(r.T(), r.client, rancher, terraform, terratest, testUser, testPassword, r.terraformOptions, configMap, newFile, rootBody, file)
			provisioning.VerifyClustersState(r.T(), r.client, clusterIDs)
		})

		params := tfpQase.GetProvisioningSchemaParams(configMap[0])
		err = qase.UpdateSchemaParameters(tt.name, params)
		if err != nil {
			logrus.Warningf("Failed to upload schema parameters %s", err)
		}
	}

	if r.terratestConfig.LocalQaseReporting {
		results.ReportTest(r.terratestConfig)
	}
}

func TestTfpRancher2RecurringRunsTestSuite(t *testing.T) {
	suite.Run(t, new(TfpRancher2RecurringRunsTestSuite))
}
