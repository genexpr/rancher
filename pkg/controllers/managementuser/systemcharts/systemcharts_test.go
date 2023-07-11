package systemcharts

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	defClusterName  = "test-cluster"
	nonRancherChart = "other-chart"
	basicValues     = "newKey: newValue"
)

var errTest = fmt.Errorf("test error")

type mockController struct {
	dsConfigClient *fake.MockClientInterface[*v1.ConfigMap, *v1.ConfigMapList]
	dsConfigCache  *fake.MockCacheInterface[*v1.ConfigMap]
}

func (m *mockController) Controller(clusterName string) *controller {
	return &controller{
		clusterName:    clusterName,
		dsConfigClient: m.dsConfigClient,
		dsConfigCache:  m.dsConfigCache,
	}
}

func Test_controller_onClusterChange(t *testing.T) {
	emptyConfigMap := &v1.ConfigMap{}
	populatedConfigMap := &v1.ConfigMap{}
	populatedConfigMap.Data = map[string]string{
		chart.WebhookChartName: "oldKey: oldValue",
		nonRancherChart:        "otherKey:otherValues",
	}
	tests := []struct {
		cluster     *v3.Cluster
		setup       func(mockController)
		name        string
		clusterName string
		wantErr     bool
	}{
		{
			name: "config map does not exist",
			cluster: defaultCluster(map[string]string{
				chart.WebhookChartName: basicValues,
			}),
			clusterName: defClusterName,
			setup: func(mocks mockController) {
				mocks.dsConfigCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(nil, errors.NewNotFound(schema.GroupResource{}, chart.CustomValueMapName))
				mocks.dsConfigClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					require.NotNil(t, obj, "unexpected nil object created")
					setValues, ok := obj.Data[chart.WebhookChartName]
					require.True(t, ok, "configMap missing webhook key")
					require.Equal(t, basicValues, setValues, "unexpected webhook values set")
					return obj, nil
				})
			},
		},
		{
			name:        "config map does not exist nil chartValues",
			cluster:     defaultCluster(nil),
			clusterName: defClusterName,
		},
		{
			name: "config map update webhook chart only",
			cluster: defaultCluster(map[string]string{
				chart.WebhookChartName: basicValues,
			}),
			clusterName: defClusterName,
			setup: func(mocks mockController) {
				mocks.dsConfigCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(populatedConfigMap.DeepCopy(), nil)
				mocks.dsConfigClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					require.NotNil(t, obj, "unexpected nil object created")
					setValues, found := obj.Data[chart.WebhookChartName]
					require.True(t, found, "configMap missing webhook key")
					require.Equal(t, basicValues, setValues, "unexpected webhook values set")

					// check other values are unchanged
					setValues, found = obj.Data[nonRancherChart]
					require.True(t, found, "configMap missing non webhook key")
					require.Equal(t, populatedConfigMap.Data[nonRancherChart], setValues, "unexpected non webhook values")
					return obj, nil
				})
			},
		},
		{
			name: "empty config map update",
			cluster: defaultCluster(map[string]string{
				chart.WebhookChartName: basicValues,
			}),
			clusterName: defClusterName,
			setup: func(mocks mockController) {
				mocks.dsConfigCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(emptyConfigMap.DeepCopy(), nil)
				mocks.dsConfigClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v1.ConfigMap) (*v1.ConfigMap, error) {
					require.NotNil(t, obj, "unexpected nil object created")
					setValues, ok := obj.Data[chart.WebhookChartName]
					require.True(t, ok, "configMap missing webhook key")
					require.Equal(t, basicValues, setValues, "unexpected webhook values set")
					return obj, nil
				})
			},
		},
		{
			name:        "config map no change",
			cluster:     defaultCluster(populatedConfigMap.Data),
			clusterName: defClusterName,
			setup: func(mocks mockController) {
				mocks.dsConfigCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(populatedConfigMap.DeepCopy(), nil)
			},
		},
		{
			name:        "config map failed get",
			wantErr:     true,
			cluster:     defaultCluster(populatedConfigMap.Data),
			clusterName: defClusterName,
			setup: func(mocks mockController) {
				mocks.dsConfigCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(nil, errTest)
			},
		},
		{
			name:        "config map failed create",
			wantErr:     true,
			cluster:     defaultCluster(populatedConfigMap.Data),
			clusterName: defClusterName,
			setup: func(mocks mockController) {
				mocks.dsConfigCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(nil, errors.NewNotFound(schema.GroupResource{}, chart.CustomValueMapName))
				mocks.dsConfigClient.EXPECT().Create(gomock.Any()).Return(nil, errTest)
			},
		},
		{
			name:    "config map failed update",
			wantErr: true,
			cluster: defaultCluster(map[string]string{
				chart.WebhookChartName: basicValues,
			}),
			clusterName: defClusterName,
			setup: func(mocks mockController) {
				mocks.dsConfigCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(populatedConfigMap.DeepCopy(), nil)
				mocks.dsConfigClient.EXPECT().Update(gomock.Any()).Return(nil, errTest)
			},
		},
		{
			name: "cluster name mismatch",
			cluster: defaultCluster(map[string]string{
				chart.WebhookChartName: basicValues,
			}),
			clusterName: "non-matching-name",
		},
		// cluster name mismatch
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mocks := mockController{
				dsConfigClient: fake.NewMockClientInterface[*v1.ConfigMap, *v1.ConfigMapList](ctrl),
				dsConfigCache:  fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
			}
			if tt.setup != nil {
				tt.setup(mocks)
			}
			c := mocks.Controller(tt.clusterName)
			got, err := c.onClusterChange("", tt.cluster)
			if tt.wantErr {
				require.Error(t, err, "missing expected error")
				return
			}
			require.NoError(t, err, "unexpected error returned")
			require.Equal(t, tt.cluster, got, "unexpected mutation to cluster object")
		})
	}
}

func defaultCluster(chartValues map[string]string) *v3.Cluster {
	testCluster := &v3.Cluster{}
	testCluster.Name = defClusterName
	testCluster.Spec.ChartValues = chartValues
	return testCluster
}
