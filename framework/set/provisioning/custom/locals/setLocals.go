package locals

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/clustertypes"
	"github.com/rancher/tfp-automation/framework/set/defaults"
	"github.com/zclconf/go-cty/cty"
)

const (
	noProxy = "localhost,127.0.0.0/8,10.0.0.0/8,172.0.0.0/8,192.168.0.0/16,.svc,.cluster.local,cattle-system.svc,169.254.169.25"
)

// SetLocals is a function that will set the locals configurations in the main.tf file.
func SetLocals(rootBody *hclwrite.Body, terraformConfig *config.TerraformConfig, terratestConfig *config.TerratestConfig,
	configMap []map[string]any, newFile *hclwrite.File, file *os.File, customClusterNames []string) (*os.File, error) {
	localsBlock := rootBody.AppendNewBlock(defaults.Locals, nil)
	localsBlockBody := localsBlock.Body()

	var roleFlags []cty.Value
	for range terratestConfig.EtcdCount {
		roleFlags = append(roleFlags, cty.StringVal(defaults.EtcdRoleFlag))
	}

	for range terratestConfig.ControlPlaneCount {
		roleFlags = append(roleFlags, cty.StringVal(defaults.ControlPlaneRoleFlag))
	}

	for range terratestConfig.WorkerCount {
		roleFlags = append(roleFlags, cty.StringVal(defaults.WorkerRoleFlag))
	}

	localsBlockBody.SetAttributeValue(defaults.RoleFlags, cty.ListVal(roleFlags))

	totalNodeCount := terratestConfig.EtcdCount + terratestConfig.ControlPlaneCount + terratestConfig.WorkerCount
	resourcePrefixExpression := fmt.Sprintf(`[for i in range(%d) : "%s-${i}"]`, totalNodeCount, terraformConfig.ResourcePrefix)
	resourcePrefixValue := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(resourcePrefixExpression)},
	}

	localsBlockBody.SetAttributeRaw(defaults.ResourcePrefix, resourcePrefixValue)

	if !strings.Contains(terraformConfig.Module, clustertypes.RKE1) {
		setV2ClusterLocalBlock(localsBlockBody, terraformConfig, customClusterNames)
	}

	return file, nil
}

func setV2ClusterLocalBlock(localsBlockBody *hclwrite.Body, terraformConfig *config.TerraformConfig, customClusterNames []string) {
	for _, name := range customClusterNames {
		setCustomClusterLocalBlock(localsBlockBody, name, terraformConfig)
	}

	//Temporary workaround until fetching insecure node command is available for rancher2_cluster_v2 resoureces with tfp-rancher2
	if strings.Contains(terraformConfig.Module, defaults.Custom) || strings.Contains(terraformConfig.Module, defaults.Airgap) {
		setCustomClusterLocalBlock(localsBlockBody, terraformConfig.ResourcePrefix, terraformConfig)

	}
}

func setCustomClusterLocalBlock(localsBlockBody *hclwrite.Body, name string, terraformConfig *config.TerraformConfig) {
	originalNodeCommandExpression := defaults.ClusterV2 + "." + name + "." + defaults.ClusterRegistrationToken + "[0]." + defaults.NodeCommand
	originalNodeCommand := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(originalNodeCommandExpression)},
	}

	localsBlockBody.SetAttributeRaw(name+"_"+defaults.OriginalNodeCommand, originalNodeCommand)

	windowsOriginalNodeCommandExpression := defaults.ClusterV2 + "." + name + "." + defaults.ClusterRegistrationToken + "[0]." + defaults.WindowsNodeCommand
	windowsOriginalNodeCommand := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(windowsOriginalNodeCommandExpression)},
	}

	localsBlockBody.SetAttributeRaw(name+"_"+defaults.WindowsOriginalNodeCommand, windowsOriginalNodeCommand)

	insecureNodeCommandExpression := fmt.Sprintf(`"${replace(local.%s_original_node_command, "curl", "curl --insecure")}"`, name)
	insecureNodeCommand := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(insecureNodeCommandExpression)},
	}

	localsBlockBody.SetAttributeRaw(name+"_"+defaults.InsecureNodeCommand, insecureNodeCommand)

	windowsInsecureNodeCommandExpression := fmt.Sprintf(`"${replace(local.%s_windows_original_node_command, "curl.exe", "curl.exe --insecure")}"`, name)
	windowsInsecureNodeCommand := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(windowsInsecureNodeCommandExpression)},
	}

	localsBlockBody.SetAttributeRaw(name+"_"+defaults.InsecureWindowsNodeCommand, windowsInsecureNodeCommand)

	if strings.Contains(terraformConfig.Module, clustertypes.WINDOWS) && (terraformConfig.Proxy != nil && terraformConfig.Proxy.ProxyBastion != "") {
		setWindowsProxyLocalBlock(localsBlockBody, name)
	}
}

func setWindowsProxyLocalBlock(localsBlockBody *hclwrite.Body, name string) error {
	// Terraform, by design, results to a .cmd file. Need to explictily call powershell.exe
	envReplace := fmt.Sprintf(`replace(local.%s_windows_original_node_command, "$env:", "powershell.exe $env:")`, name)
	curlReplace := fmt.Sprintf(`"${replace(%s, "curl.exe", "curl.exe --insecure")}"`, envReplace)

	proxyWindowsInsecureNodeCommand := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(curlReplace)},
	}

	localsBlockBody.SetAttributeRaw(name+"_"+defaults.InsecureWindowsProxyNodeCommand, proxyWindowsInsecureNodeCommand)

	return nil
}
