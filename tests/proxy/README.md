# Proxy Provisioning Tests

In the tfp-automation proxy provisioning test, the following workflow is followed:

1. Setup proxy Rancher HA utilizing Terraform resources + specified provider infrastructure
2. Provision downstream RKE1 / RKE2 / K3S clusters - with the proxy and without the proxy
3. Perform post-cluster provisioning checks
4. Cleanup resources (Terraform explicitly needs to call its cleanup method so that each test doesn't experience caching issues)

Please see below for more details for your config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

## Table of Contents
1. [Getting Started](#Getting-Started)
2. [Local Qase Reporting](#Local-Qase-Reporting)

## Getting Started
The config is split up into multiple parts. Think of the parts as follows:
- Standalone config for setting up Rancher
- Standalone config for setting up private registry
- Custom cluster config for provisioning downstream clusters
- Rancher config

In no particular order, see an example below:

```yaml
#######################
# RANCHER CONFIG
#######################
rancher:
  host: ""                                        # REQUIRED - fill out with the expected Rancher server URL
  adminPassword: ""                               # REQUIRED - this is the same as the bootstrapPassword below, make sure they match
  adminToken: ""                                  # REQUIRED - leave this field empty as shown
  insecure: true                                  # REQUIRED - leave this as true
#######################
# TERRAFORM CONFIG
#######################
terraform:
  cni: ""                                         # REQUIRED - fill with desired value
  defaultClusterRoleForProjectMembers: "true"     # REQUIRED - leave value as true
  enableNetworkPolicy: false                      # REQUIRED - values are true or false -  can leave as false
  privateKeyPath: ""                              # REQUIRED - specify private key that will be used to access created instances
  provider: "aws"
  proxy:
    proxyBastion: ""                              # REQUIRED - this will be set/unset during testing
  resourcePrefix: ""                              # REQUIRED - fill with desired value
  windowsPrivateKeyPath: ""                       # REQUIRED - specify Windows private key that will be used to access created instances
  ########################
  # INFRASTRUCTURE SETUP
  ########################
  awsCredentials:
    awsAccessKey: ""
    awsSecretKey: ""
  awsConfig:
    ami: ""
    awsKeyName: ""
    awsInstanceType: ""
    awsSecurityGroups: [""]
    awsSecurityGroupNames: [""]
    awsSubnetID: ""
    awsVpcID: ""
    awsZoneLetter: ""
    awsRootSize: 100
    awsRoute53Zone: ""
    region: ""
    awsUser: ""
    sshConnectionType: "ssh"
    timeout: "5m"
    ipAddressType: "ipv4"
    loadBalancerType: "ipv4"
    targetType: "instance"
    windowsAMI2019: ""
    windowsAMI2022: ""
    windowsAWSUser: ""
    windows2019Password: ""
    windows2022Password: ""
    windowsInstanceType: ""
    windowsKeyName: ""
  ###################################
  # STANDALONE CONFIG - RANCHER SETUP
  ###################################
  standalone:
    bootstrapPassword: ""                         # REQUIRED - this is the same as the adminPassword above, make sure they match
    certManagerVersion: ""                        # REQUIRED - (e.g. v1.15.3)
    certType: ""                                  # REQUIRED - "self-signed" or "lets-encrypt"
    chartVersion: ""                              # REQUIRED - fill with desired value (leave out the leading 'v')
    osGroup: ""                                   # REQUIRED - fill with desired value
    osUser: ""                                    # REQUIRED - fill with desired value
    rancherAgentImage: ""                         # OPTIONAL - fill out only if you are using Rancher Prime or staging registry
    rancherChartVersion: ""                       # REQUIRED - fill with desired value
    rancherChartRepository: ""                    # REQUIRED - fill with desired value. Must end with a trailing /
    rancherHostname: ""                           # REQUIRED - fill with desired value
    rancherImage: ""                              # REQUIRED - fill with desired value
    rancherTagVersion: ""                         # REQUIRED - fill with desired value
    repo: ""                                      # REQUIRED - fill with desired value
    rke2Group: ""                                 # REQUIRED - fill with group of the instance created
    rke2User: ""                                  # REQUIRED - fill with username of the instance created
    rke2Version: ""                               # REQUIRED - fill with desired RKE2 k8s value (i.e. v1.30.9)
    upgradedRancherAgentImage: ""                 # OPTIONAL - fill out if you are performing an upgrade
    upgradedRancherChartRepository: ""            # OPTIONAL - fill out if you are performing an upgrade
    upgradedRancherChartVersion: ""               # OPTIONAL - fill out if you are performing an upgrade
    upgradedRancherImage: ""                      # OPTIONAL - fill out if you are performing an upgrade
    upgradedRancherRepo: ""                       # OPTIONAL - fill out if you are performing an upgrade
    upgradedRancherTagVersion: ""                 # OPTIONAL - fill out if you are performing an upgrade
  ####################################
  # STANDALONE CONFIG - REGISTRY SETUP
  ####################################
  standaloneRegistry:
    registryName: ""                              # REQUIRED - fill with desired value
    registryPassword: ""                          # REQUIRED - fill with desired value
    registryUsername: ""                          # REQUIRED - fill with desired value
#######################
# TERRATEST CONFIG
#######################
terratest:  
  nodeCount: 3
  windowsNodeCount: 1
  pathToRepo: "go/src/github.com/rancher/tfp-automation"
```

Before running, be sure to run the following commands:

```yaml
export RANCHER2_PROVIDER_VERSION=""
export CATTLE_TEST_CONFIG=<path/to/yaml>
export LOCALS_PROVIDER_VERSION=""
export CLOUD_PROVIDER_VERSION=""
export LETS_ENCRYPT_EMAIL=""                      # OPTIONAL - must provide a valid email address
```

See the below examples on how to run the tests:

`gotestsum --format standard-verbose --packages=github.com/rancher/tfp-automation/tests/proxy --junitfile results.xml --jsonfile results.json -- -timeout=3h -v -run "TestTfpProxyProvisioningTestSuite$"` \
`gotestsum --format standard-verbose --packages=github.com/rancher/tfp-automation/tests/proxy --junitfile results.xml --jsonfile results.json -- -timeout=3h -v -run "TestTfpProxyUpgradeRancherTestSuite$"`

If the specified test passes immediately without warning, try adding the -count=1 flag to get around this issue. This will avoid previous results from interfering with the new test run.

## Local Qase Reporting
If you are planning to report to Qase locally, then you will need to have the following done:
1. The `terratest` block in your config file must have `localQaseReporting: true`.
2. The working shell session must have the following two environmental variables set:
     - `QASE_AUTOMATION_TOKEN=""`
     - `QASE_TEST_RUN_ID=""`
3. Append `./reporter` to the end of the `gotestsum` command. See an example below::
     - `gotestsum --format standard-verbose --packages=github.com/rancher/tfp-automation/tests/proxy  --junitfile results.xml --jsonfile results.json -- -timeout=3h -v -run TestTfpProxyProvisioningTestSuite$";/path/to/tfp-automation/reporter`