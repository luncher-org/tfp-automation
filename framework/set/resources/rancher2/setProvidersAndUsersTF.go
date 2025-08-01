package rancher2

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/config/operations"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/clustertypes"
	"github.com/rancher/tfp-automation/framework/set/defaults"
	"github.com/rancher/tfp-automation/framework/set/provisioning/custom/sleep"
	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

const (
	apiURL                  = "api_url"
	alias                   = "alias"
	allowUnverifiedSSL      = "allow_unverified_ssl"
	ec2                     = "ec2"
	globalRoleBinding       = "rancher2_global_role_binding"
	globalRoleID            = "global_role_id"
	insecure                = "insecure"
	name                    = "name"
	password                = "password"
	provider                = "provider"
	rancher2                = "rancher2"
	rancher2CustomUserToken = "rancher2_custom_user_token"
	rancherRKE              = "rancher/rke"
	rancherSource           = "source"
	rancherUser             = "rancher2_user"
	rc                      = "-rc"
	requiredProviders       = "required_providers"
	terraform               = "terraform"
	testPassword            = "password"
	tokenKey                = "token_key"
	ttl                     = "ttl"
	version                 = "version"
	vsphere                 = "vsphere"
	user                    = "user"
	userID                  = "user_id"
	username                = "username"
	providerEnvVar          = "RANCHER2_PROVIDER_VERSION"
	cloudProviderEnvVar     = "CLOUD_PROVIDER_VERSION"
	localProviderEnvVar     = "LOCALS_PROVIDER_VERSION"
	rkeEnvVar               = "RKE_PROVIDER_VERSION"
)

// SetProvidersAndUsersTF is a helper function that will set the general Terraform configurations in the main.tf file.
func SetProvidersAndUsersTF(rancherConfig *rancher.Config, testUser, testPassword string, authProvider bool, newFile *hclwrite.File, rootBody *hclwrite.Body,
	configMap []map[string]any, customModule bool) (*hclwrite.File, *hclwrite.Body) {
	createRequiredProviders(rootBody, configMap, customModule)
	createProvider(rancherConfig, rootBody, configMap, customModule)
	createProviderAlias(rancherConfig, rootBody)
	createUser(rootBody, testUser, testPassword)

	if !authProvider {
		createGlobalRoleBinding(rootBody, testUser, userID, configMap)
	}

	createCustomUserToken(rootBody)

	return newFile, rootBody
}

// createRequiredProviders creates the required_providers block.
func createRequiredProviders(rootBody *hclwrite.Body, configMap []map[string]any, customModule bool) {
	tfBlock := rootBody.AppendNewBlock(terraform, nil)
	tfBlockBody := tfBlock.Body()

	reqProvsBlock := tfBlockBody.AppendNewBlock(requiredProviders, nil)
	reqProvsBlockBody := reqProvsBlock.Body()

	terraformConfig := new(config.TerraformConfig)
	operations.LoadObjectFromMap(config.TerraformConfigurationFileKey, configMap[0], terraformConfig)

	source, rancherProviderVersion, cloudProviderVersion, localProviderVersion, rkeProviderVersion := getRequiredProviderVersions(configMap)

	if rancherProviderVersion != "" {
		reqProvsBlockBody.SetAttributeValue(rancher2, cty.ObjectVal(map[string]cty.Value{
			rancherSource: cty.StringVal(source),
			version:       cty.StringVal(rancherProviderVersion),
		}))
	}

	if cloudProviderVersion != "" && terraformConfig.Provider == defaults.Aws && customModule {
		reqProvsBlockBody.SetAttributeValue(defaults.Aws, cty.ObjectVal(map[string]cty.Value{
			defaults.Source:  cty.StringVal(defaults.AwsSource),
			defaults.Version: cty.StringVal(cloudProviderVersion),
		}))
	}

	if cloudProviderVersion != "" && terraformConfig.Provider == defaults.Linode && customModule {
		reqProvsBlockBody.SetAttributeValue(defaults.Linode, cty.ObjectVal(map[string]cty.Value{
			defaults.Source:  cty.StringVal(defaults.LinodeSource),
			defaults.Version: cty.StringVal(cloudProviderVersion),
		}))
	}

	if cloudProviderVersion != "" && terraformConfig.Provider == defaults.Vsphere && customModule {
		reqProvsBlockBody.SetAttributeValue(defaults.Vsphere, cty.ObjectVal(map[string]cty.Value{
			defaults.Source:  cty.StringVal(defaults.VsphereSource),
			defaults.Version: cty.StringVal(cloudProviderVersion),
		}))
	}

	if localProviderVersion != "" {
		reqProvsBlockBody.SetAttributeValue(defaults.Local, cty.ObjectVal(map[string]cty.Value{
			defaults.Source:  cty.StringVal(defaults.LocalSource),
			defaults.Version: cty.StringVal(localProviderVersion),
		}))
	}

	if rkeProviderVersion != "" {
		reqProvsBlockBody.SetAttributeValue(defaults.RKE, cty.ObjectVal(map[string]cty.Value{
			defaults.Source:  cty.StringVal(rancherRKE),
			defaults.Version: cty.StringVal(rkeProviderVersion),
		}))
	}

	rootBody.AppendNewline()
}

// createProvider creates a provider block for the given rancher config.
func createProvider(rancherConfig *rancher.Config, rootBody *hclwrite.Body, configMap []map[string]any, customModule bool) {
	_, _, cloudProviderVersion, _, _ := getRequiredProviderVersions(configMap)

	terraformConfig := new(config.TerraformConfig)
	operations.LoadObjectFromMap(config.TerraformConfigurationFileKey, configMap[0], terraformConfig)

	if cloudProviderVersion != "" && terraformConfig.Provider == defaults.Aws && customModule {
		awsProvBlock := rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Aws})
		awsProvBlockBody := awsProvBlock.Body()

		awsProvBlockBody.SetAttributeValue(defaults.Region, cty.StringVal(terraformConfig.AWSConfig.Region))
		awsProvBlockBody.SetAttributeValue(defaults.AccessKey, cty.StringVal(terraformConfig.AWSCredentials.AWSAccessKey))
		awsProvBlockBody.SetAttributeValue(defaults.SecretKey, cty.StringVal(terraformConfig.AWSCredentials.AWSSecretKey))

		rootBody.AppendNewline()
		rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Local})
		rootBody.AppendNewline()
	}

	if cloudProviderVersion != "" && terraformConfig.Provider == defaults.Linode && customModule {
		linodeProvBlock := rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Linode})
		linodeProvBlockBody := linodeProvBlock.Body()

		linodeProvBlockBody.SetAttributeValue(defaults.Token, cty.StringVal(terraformConfig.LinodeCredentials.LinodeToken))

		rootBody.AppendNewline()
		rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Local})
		rootBody.AppendNewline()
	}

	if cloudProviderVersion != "" && terraformConfig.Provider == defaults.Vsphere && customModule {
		vsphereProvBlock := rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Vsphere})
		vsphereProvBlockBody := vsphereProvBlock.Body()

		vsphereProvBlockBody.SetAttributeValue(defaults.User, cty.StringVal(terraformConfig.VsphereCredentials.Username))
		vsphereProvBlockBody.SetAttributeValue(defaults.Password, cty.StringVal(terraformConfig.VsphereCredentials.Password))
		vsphereProvBlockBody.SetAttributeValue(defaults.VsphereServer, cty.StringVal(terraformConfig.VsphereCredentials.Vcenter))
		vsphereProvBlockBody.SetAttributeValue(allowUnverifiedSSL, cty.BoolVal(true))

		rootBody.AppendNewline()
		rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Local})
		rootBody.AppendNewline()
	}

	rancher2ProvBlock := rootBody.AppendNewBlock(provider, []string{rancher2})
	rancher2ProvBlockBody := rancher2ProvBlock.Body()

	rancher2ProvBlockBody.SetAttributeValue(apiURL, cty.StringVal("https://"+rancherConfig.Host))
	rancher2ProvBlockBody.SetAttributeValue(tokenKey, cty.StringVal(rancherConfig.AdminToken))
	rancher2ProvBlockBody.SetAttributeValue(insecure, cty.BoolVal(*rancherConfig.Insecure))

	rootBody.AppendNewline()
}

// createProviderAlias creates a provider alias block for the standard user.
func createProviderAlias(rancherConfig *rancher.Config, rootBody *hclwrite.Body) {
	providerBlock := rootBody.AppendNewBlock(defaults.Provider, []string{rancher2})
	providerBlockBody := providerBlock.Body()

	providerBlockBody.SetAttributeValue(alias, cty.StringVal(defaults.StandardUser))
	providerBlockBody.SetAttributeValue(apiURL, cty.StringVal("https://"+rancherConfig.Host))

	customToken := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(rancher2CustomUserToken + "." + rancherUser + ".token")},
	}

	providerBlockBody.SetAttributeRaw(tokenKey, customToken)
	providerBlockBody.SetAttributeValue(insecure, cty.BoolVal(true))

	rootBody.AppendNewline()
}

// createUser creates the user block for a new user.
func createUser(rootBody *hclwrite.Body, testUser, testpassword string) {
	userBlock := rootBody.AppendNewBlock(defaults.Resource, []string{rancherUser, rancherUser})
	userBlockBody := userBlock.Body()

	userBlockBody.SetAttributeValue(name, cty.StringVal(testUser))
	userBlockBody.SetAttributeValue(username, cty.StringVal(testUser))
	userBlockBody.SetAttributeValue(testPassword, cty.StringVal(testpassword))
	userBlockBody.SetAttributeValue(defaults.Enabled, cty.BoolVal(true))

	rootBody.AppendNewline()
}

// createGlobalRoleBinding creates a global role binding block for the given user.
func createGlobalRoleBinding(rootBody *hclwrite.Body, testUser string, userID string, configMap []map[string]any) {
	terraformConfig := new(config.TerraformConfig)
	operations.LoadObjectFromMap(config.TerraformConfigurationFileKey, configMap[0], terraformConfig)

	globalRoleBindingBlock := rootBody.AppendNewBlock(defaults.Resource, []string{globalRoleBinding, globalRoleBinding})
	globalRoleBindingBlockBody := globalRoleBindingBlock.Body()

	globalRoleBindingBlockBody.SetAttributeValue(name, cty.StringVal(testUser))
	globalRoleBindingBlockBody.SetAttributeValue(globalRoleID, cty.StringVal(user))

	standardUser := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(rancherUser + "." + rancherUser + ".id")},
	}

	globalRoleBindingBlockBody.SetAttributeRaw(userID, standardUser)

	dependsOnValue := fmt.Sprintf("[" + globalRoleBinding + "." + globalRoleBinding + "]")

	rootBody.AppendNewline()
	sleep.SetTimeSleep(rootBody, terraformConfig, "5s", dependsOnValue)
	rootBody.AppendNewline()
}

// createCustomUserToken creates a custom user token for the given user.
func createCustomUserToken(rootBody *hclwrite.Body) {
	customTokenBlock := rootBody.AppendNewBlock(defaults.Resource, []string{rancher2CustomUserToken, rancherUser})
	customTokenBlockBody := customTokenBlock.Body()

	standardUser := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(rancherUser + "." + rancherUser + ".username")},
	}

	standardUserPassword := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(rancherUser + "." + rancherUser + ".password")},
	}

	dependsOnBlock := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte("[" + globalRoleBinding + "." + globalRoleBinding + "]")},
	}

	customTokenBlockBody.SetAttributeRaw(username, standardUser)
	customTokenBlockBody.SetAttributeRaw(password, standardUserPassword)
	customTokenBlockBody.SetAttributeRaw(defaults.DependsOn, dependsOnBlock)
	customTokenBlockBody.SetAttributeValue(ttl, cty.NumberIntVal(7776000))
}

// Determines the required providers from the list of configs.
func getRequiredProviderVersions(configMap []map[string]any) (source, rancherProviderVersion, rkeProviderVersion, localProviderVersion,
	cloudProviderVersion string) {
	for _, cattleConfig := range configMap {
		terraformConfig := new(config.TerraformConfig)
		operations.LoadObjectFromMap(config.TerraformConfigurationFileKey, cattleConfig, terraformConfig)
		module := terraformConfig.Module

		rancherProviderVersion = os.Getenv(providerEnvVar)
		if rancherProviderVersion == "" {
			logrus.Fatalf("Expected env var not set %s", providerEnvVar)
		}

		source = "rancher/rancher2"
		if strings.Contains(rancherProviderVersion, rc) {
			source = "terraform.local/local/rancher2"
		}

		if strings.Contains(module, defaults.Import) && strings.Contains(module, clustertypes.RKE1) {
			rkeProviderVersion = os.Getenv(rkeEnvVar)
			if rkeProviderVersion == "" {
				logrus.Fatalf("Expected env var not set %s", rkeEnvVar)
			}
		}

		if strings.Contains(module, defaults.Custom) || strings.Contains(module, defaults.Import) || strings.Contains(module, defaults.Airgap) ||
			strings.Contains(module, ec2) {
			cloudProviderVersion = os.Getenv(cloudProviderEnvVar)
			if cloudProviderVersion == "" {
				logrus.Fatalf("Expected env var not set %s", cloudProviderEnvVar)
			}

			localProviderVersion = os.Getenv(localProviderEnvVar)
			if localProviderVersion == "" {
				logrus.Fatalf("Expected env var not set %s", localProviderEnvVar)
			}
		}
	}

	return source, rancherProviderVersion, cloudProviderVersion, localProviderVersion, rkeProviderVersion
}
