package snapshot

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	timeouts "github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/tests/actions/services"
	deploy "github.com/rancher/tests/actions/workloads/deployment"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/clustertypes"
	"github.com/rancher/tfp-automation/defaults/stevetypes"
	framework "github.com/rancher/tfp-automation/framework/set"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	StorageAnnotation        = "etcdsnapshot.rke.io/storage"
	SnapshotAnnotation       = "rke.cattle.io.etcdsnapshot"
	SnapshotClusterNameLabel = "rke.cattle.io/cluster-name"

	active              = "active"
	all                 = "all"
	containerImage      = "nginx"
	containerName       = "nginx"
	defaultNamespace    = "default"
	DeploymentSteveType = "apps.deployment"
	initialWorkload     = "wload-before-restore"
	isCattleLabeled     = true
	localCluster        = "local"
	kubernetesVersion   = "kubernetesVersion"
	namespace           = "fleet-default"
	port                = "port"
	postWorkload        = "wload-after-backup"
	S3                  = "s3"
	serviceAppendName   = "service-"
	serviceType         = "service"
)

// snapshotRestore creates workloads, takes a snapshot of the cluster, restores the cluster and verifies the workloads created after
// a snapshot no longer are present in the cluster
func snapshotRestore(t *testing.T, client *rancher.Client, rancherConfig *rancher.Config, terraformConfig *config.TerraformConfig,
	terratestConfig *config.TerratestConfig, testUser, testPassword string, terraformOptions *terraform.Options, configMap []map[string]any) {
	initialWorkloadName := namegen.AppendRandomString(initialWorkload)

	clusterID, err := clusters.GetClusterIDByName(client, terraformConfig.ResourcePrefix)
	require.NoError(t, err)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	containerTemplate := workloads.NewContainer(containerName, containerImage, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)
	podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil, nil)

	deploymentResp, serviceResp := createWorkloads(t, client, clusterID, podTemplate, initialWorkloadName, isCattleLabeled, DeploymentSteveType)

	cluster, snapshotName, postDeploymentResp, postServiceResp, err := snapshotV2Prov(t, client, rancherConfig, terraformConfig, terratestConfig, podTemplate, testUser, testPassword, clusterID, terraformOptions, configMap)
	require.NoError(t, err)

	restoreV2Prov(t, client, rancherConfig, terraformConfig, terratestConfig, snapshotName, testUser, testPassword, cluster, clusterID, terraformOptions, configMap)

	_, err = steveclient.SteveType(DeploymentSteveType).ByID(postDeploymentResp.ID)
	require.Error(t, err)

	_, err = steveclient.SteveType(serviceType).ByID(postServiceResp.ID)
	require.Error(t, err)

	logrus.Infof("Deleting created workloads...")
	err = steveclient.SteveType(stevetypes.Deployment).Delete(deploymentResp)
	require.NoError(t, err)

	err = steveclient.SteveType(stevetypes.Service).Delete(serviceResp)
	require.NoError(t, err)
}

// snapshotV2Prov takes a snapshot of the cluster and creates a deployment and service in the cluster.
func snapshotV2Prov(t *testing.T, client *rancher.Client, rancherConfig *rancher.Config, terraformConfig *config.TerraformConfig,
	terratestConfig *config.TerratestConfig, podTemplate corev1.PodTemplateSpec, testUser, testPassword, clusterID string,
	terraformOptions *terraform.Options, configMap []map[string]any) (*apisV1.Cluster, string, *steveV1.SteveAPIObject, *steveV1.SteveAPIObject, error) {
	terratestConfig.SnapshotInput.CreateSnapshot = true

	_, err := framework.ConfigTF(nil, testUser, testPassword, "", configMap, false)
	require.NoError(t, err)

	terraform.Apply(t, terraformOptions)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	cluster, _, err := clusters.GetProvisioningClusterByName(client, terraformConfig.ResourcePrefix, namespace)
	require.NoError(t, err)

	podErrors := pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)

	postWorkloadName := namegen.AppendRandomString(postWorkload)
	postDeploymentResp, postServiceResp := createWorkloads(t, client, clusterID, podTemplate, postWorkloadName, isCattleLabeled, DeploymentSteveType)

	snapshotID, err := getSnapshots(client, terraformConfig.ResourcePrefix)
	require.NoError(t, err)

	if terratestConfig.SnapshotInput.SnapshotRestore == kubernetesVersion || terratestConfig.SnapshotInput.SnapshotRestore == all {
		upgradeCluster(t, client, rancherConfig, testUser, testPassword, clusterID, terratestConfig, terraformConfig, terraformOptions, configMap)
	}

	return cluster, snapshotID[0].Name, postDeploymentResp, postServiceResp, err
}

// restoreV2Prov restores the cluster to the previous state after a snapshot is taken.
func restoreV2Prov(t *testing.T, client *rancher.Client, rancherConfig *rancher.Config, terraformConfig *config.TerraformConfig,
	terratestConfig *config.TerratestConfig, snapshotName, testUser, testPassword string, cluster *apisV1.Cluster,
	clusterID string, terraformOptions *terraform.Options, configMap []map[string]any) {
	terratestConfig.SnapshotInput.CreateSnapshot = false
	terratestConfig.SnapshotInput.RestoreSnapshot = true
	terratestConfig.SnapshotInput.SnapshotName = snapshotName

	_, err := framework.ConfigTF(nil, testUser, testPassword, "", configMap, false)
	require.NoError(t, err)

	terraform.Apply(t, terraformOptions)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	clusterObject, _, err := clusters.GetProvisioningClusterByName(client, terraformConfig.ResourcePrefix, namespace)
	require.NoError(t, err)

	logrus.Infof("Cluster version is restored to: %s", clusterObject.Spec.KubernetesVersion)

	podErrors := pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)

	if terratestConfig.SnapshotInput.SnapshotRestore == kubernetesVersion || terratestConfig.SnapshotInput.SnapshotRestore == all {
		clusterObject, _, err := clusters.GetProvisioningClusterByName(client, terraformConfig.ResourcePrefix, namespace)
		require.NoError(t, err)
		require.Equal(t, cluster.Spec.KubernetesVersion, clusterObject.Spec.KubernetesVersion)

		if terratestConfig.SnapshotInput.ControlPlaneConcurrencyValue != "" && terratestConfig.SnapshotInput.WorkerConcurrencyValue != "" {
			logrus.Infof("Control plane concurrency value is restored to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			logrus.Infof("Worker concurrency value is restored to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)

			require.Equal(t, cluster.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			require.Equal(t, cluster.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
		}
	}
}

// upgradeCluster upgrades the cluster to the specified version.
func upgradeCluster(t *testing.T, client *rancher.Client, rancherConfig *rancher.Config, testUser, testPassword,
	clusterID string, terratestConfig *config.TerratestConfig, terraformConfig *config.TerraformConfig, terraformOptions *terraform.Options, configMap []map[string]any) {
	clusterObject, _, err := clusters.GetProvisioningClusterByName(client, terraformConfig.ResourcePrefix, namespace)
	require.NoError(t, err)

	initialKubernetesVersion := clusterObject.Spec.KubernetesVersion

	if terratestConfig.SnapshotInput.UpgradeKubernetesVersion == "" {
		if strings.Contains(initialKubernetesVersion, clustertypes.RKE2) {
			defaultVersion, err := kubernetesversions.Default(client, clusters.RKE2ClusterType.String(), nil)
			terratestConfig.SnapshotInput.UpgradeKubernetesVersion = defaultVersion[0]
			require.NoError(t, err)
		} else if strings.Contains(initialKubernetesVersion, clustertypes.K3S) {
			defaultVersion, err := kubernetesversions.Default(client, clusters.K3SClusterType.String(), nil)
			terratestConfig.SnapshotInput.UpgradeKubernetesVersion = defaultVersion[0]
			require.NoError(t, err)
		}
	}

	clusterObject.Spec.KubernetesVersion = terratestConfig.SnapshotInput.UpgradeKubernetesVersion

	if terratestConfig.SnapshotInput.SnapshotRestore == all && terratestConfig.SnapshotInput.ControlPlaneConcurrencyValue != "" && terratestConfig.SnapshotInput.WorkerConcurrencyValue != "" {
		clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency = terratestConfig.SnapshotInput.ControlPlaneConcurrencyValue
		clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency = terratestConfig.SnapshotInput.WorkerConcurrencyValue
	}

	terratestConfig.KubernetesVersion = clusterObject.Spec.KubernetesVersion
	terratestConfig.SnapshotInput.CreateSnapshot = false

	_, err = framework.ConfigTF(nil, testUser, testPassword, "", configMap, false)
	require.NoError(t, err)

	terraform.Apply(t, terraformOptions)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	logrus.Infof("Cluster version is upgraded to: %s", clusterObject.Spec.KubernetesVersion)

	podErrors := pods.StatusPods(client, clusterID)
	assert.Empty(t, podErrors)
	require.Equal(t, terratestConfig.SnapshotInput.UpgradeKubernetesVersion, clusterObject.Spec.KubernetesVersion)

	if terratestConfig.SnapshotInput.SnapshotRestore == all && terratestConfig.SnapshotInput.ControlPlaneConcurrencyValue != "" && terratestConfig.SnapshotInput.WorkerConcurrencyValue != "" {
		logrus.Infof("Control plane concurrency value is set to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
		logrus.Infof("Worker concurrency value is set to: %s", clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)

		require.Equal(t, terratestConfig.SnapshotInput.ControlPlaneConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
		require.Equal(t, terratestConfig.SnapshotInput.WorkerConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
	}
}

// getSnapshots retrieves all snapshots for a given cluster.
func getSnapshots(client *rancher.Client, clusterName string) ([]steveV1.SteveAPIObject, error) {
	localclusterID, err := clusters.GetClusterIDByName(client, localCluster)
	if err != nil {
		return nil, err
	}

	steveclient, err := client.Steve.ProxyDownstream(localclusterID)
	if err != nil {
		return nil, err
	}

	snapshotSteveObjList, err := steveclient.SteveType(SnapshotAnnotation).List(nil)
	if err != nil {
		return nil, err
	}

	snapshots := []steveV1.SteveAPIObject{}
	for _, snapshot := range snapshotSteveObjList.Data {
		if strings.Contains(snapshot.ObjectMeta.Name, clusterName) {
			snapshots = append(snapshots, snapshot)
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].ObjectMeta.CreationTimestamp.Before(&snapshots[j].ObjectMeta.CreationTimestamp)
	})

	return snapshots, nil
}

// createWorkloads creates a deployment and service in a given cluster and verifies they are active.
func createWorkloads(t *testing.T, client *rancher.Client, clusterID string, podTemplate corev1.PodTemplateSpec, workloadName string, isCattleLabeled bool, deploymentType string) (*steveV1.SteveAPIObject, *steveV1.SteveAPIObject) {
	deployment := workloads.NewDeploymentTemplate(workloadName, defaultNamespace, podTemplate, isCattleLabeled, nil)

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAppendName + workloadName,
			Namespace: defaultNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: port,
					Port: 80,
				},
			},
			Selector: deployment.Spec.Template.Labels,
		},
	}

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	deploymentResp, err := steveclient.SteveType(deploymentType).Create(deployment)
	require.NoError(t, err)

	err = kwait.PollUntilContextTimeout(context.TODO(), timeouts.FiveSecondTimeout, timeouts.FiveMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		deployment, err := client.Steve.SteveType(deploymentType).ByID(deploymentResp.ID)
		if err != nil {
			return false, err
		}

		if deployment.State.Name == active {
			logrus.Infof("%s(%s) is active", deploymentType, deployment.Name)
			return true, nil
		}

		return false, nil
	})

	err = deploy.VerifyDeployment(steveclient, deploymentResp)
	require.NoError(t, err)
	require.Equal(t, workloadName, deploymentResp.ObjectMeta.Name)

	serviceResp, err := services.CreateService(steveclient, service)
	require.NoError(t, err)

	err = services.VerifyService(steveclient, serviceResp)
	require.NoError(t, err)
	require.Equal(t, serviceAppendName+workloadName, serviceResp.ObjectMeta.Name)

	return deploymentResp, serviceResp
}
