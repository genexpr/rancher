// Package systemcharts hold a controller used to managed systemcharts in the UserContext.
package systemcharts

import (
	"context"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var configHandlerName = "systemcharts-config-sync"

type controller struct {
	dsConfigClient wcorev1.ConfigMapClient
	dsConfigCache  wcorev1.ConfigMapCache
	clusterName    string
}

// Register registers the handlers needed for the systemcharts controller.
func Register(ctx context.Context, cluster *config.UserContext) {
	ctrl := controller{
		clusterName:    cluster.ClusterName,
		dsConfigClient: cluster.Corew.ConfigMap(),
		dsConfigCache:  cluster.Corew.ConfigMap().Cache(),
	}
	cluster.Management.Wrangler.Mgmt.Cluster().OnChange(ctx, configHandlerName, ctrl.onClusterChange)
}

// onClusterChange handler watches for changes to the v3.Cluster in the local cluster and syncs the updated configMap to the downstream cluster.
func (c *controller) onClusterChange(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.Name != c.clusterName || cluster.Spec.ChartValues == nil {
		return cluster, nil
	}
	webhookValues := cluster.Spec.ChartValues[chart.WebhookChartName]

	// using a cache here means we are indexing all of the configMaps on all downstream clusters.
	// currently the indexer is already in use for the snapshotbackpopulate controller.
	// if that controller is ever removed this can be switched to use a client for reduced memory usage.
	clusterConfigMap, err := c.dsConfigCache.Get(namespace.System, chart.CustomValueMapName)
	if errors.IsNotFound(err) {
		// create a new configMap if there isn't one already
		if err := c.createConfigMap(webhookValues); err != nil {
			return nil, err
		}
		return cluster, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve ConfigMap '%s' on cluster '%s': %w", chart.CustomValueMapName, c.clusterName, err)
	}
	if clusterConfigMap.Data == nil {
		clusterConfigMap.Data = map[string]string{}
	} else if clusterConfigMap.Data[chart.WebhookChartName] == webhookValues {
		// values have not changed there is no need to update anything.
		return cluster, nil
	}
	clusterConfigMap.Data[chart.WebhookChartName] = webhookValues
	_, err = c.dsConfigClient.Update(clusterConfigMap)
	if err != nil {
		return nil, fmt.Errorf("failed to update '%s' ConfigMap: %w", chart.CustomValueMapName, err)
	}
	return cluster, nil
}

// createConfigMap creates a new configMap with specified webhookValues.
func (c *controller) createConfigMap(webhookValues string) error {
	clusterConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chart.CustomValueMapName,
			Namespace: namespace.System,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "rancher",
			},
		},
		Data: map[string]string{chart.WebhookChartName: webhookValues},
	}
	_, err := c.dsConfigClient.Create(clusterConfigMap)
	if err != nil {
		return fmt.Errorf("failed to create '%s' ConfigMap: %w", chart.CustomValueMapName, err)
	}
	return nil
}
