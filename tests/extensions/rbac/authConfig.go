package rbac

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/tfp-automation/config"
	framework "github.com/rancher/tfp-automation/framework/set"
	"github.com/stretchr/testify/require"
)

// AuthConfig is a function that will run terraform apply to setup authentication providers.
func AuthConfig(t *testing.T, terraformConfig *config.TerraformConfig, terraformOptions *terraform.Options, testUser, testPassword string,
	configMap []map[string]any) {
	isSupported := SupportedAuthProviders(terraformConfig, terraformOptions)
	require.True(t, isSupported)

	err := framework.AuthConfig(testUser, testPassword, configMap)
	require.NoError(t, err)

	terraform.InitAndApply(t, terraformOptions)
}
