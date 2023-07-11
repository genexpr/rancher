package systemcharts

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	chartfake "github.com/rancher/rancher/pkg/controllers/dashboard/chart/fake"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/generic/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errTest           = fmt.Errorf("test error")
	priorityClassName = "rancher-critical"
	operatorNamespace = "rancher-operator-system"
	priorityConfig    = &v1.ConfigMap{
		Data: map[string]string{
			"priorityClassName": priorityClassName,
		},
	}
	fullConfig = &v1.ConfigMap{
		Data: map[string]string{
			"priorityClassName":    priorityClassName,
			chart.WebhookChartName: testYAML,
		},
	}
	invalidConfig = &v1.ConfigMap{
		Data: map[string]string{
			chart.WebhookChartName: "--- %{}---\n:",
		},
	}
	emptyConfig        = &v1.ConfigMap{}
	originalMinVersion = settings.RancherWebhookMinVersion.Get()
	originalVersion    = settings.RancherWebhookVersion.Get()
)

var testYAML = `---
newKey: newValue
mcm:
  enabled: false
global: ""
`

type testMocks struct {
	manager       *chartfake.MockManager
	namespaceCtrl *fake.MockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList]
	configCache   *fake.MockCacheInterface[*v1.ConfigMap]
}

func (t *testMocks) Handler() *handler {
	return &handler{
		manager:      t.manager,
		namespaces:   t.namespaceCtrl,
		chartsConfig: chart.RancherConfigGetter{ConfigCache: t.configCache},
	}
}

// Test_ChartInstallation test that all expected charts are installed or uninstalled with expected configuration.
func Test_ChartInstallation(t *testing.T) {
	repo := &catalog.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: repoName,
		},
	}

	tests := []struct {
		name             string
		setup            func(testMocks)
		registryOverride string
		wantErr          bool
	}{
		{
			name: "normal installation",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(priorityConfig, nil).Times(2)
				settings.RancherWebhookVersion.Set("2.0.0")
				expectedValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
		},
		{
			name: "installation without webhook priority class",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(nil, errTest).Times(2)
				settings.RancherWebhookVersion.Set("2.0.0")
				expectedValues := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
		},
		{
			name: "installation with image override",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(emptyConfig, nil).Times(2)
				settings.RancherWebhookVersion.Set("2.0.1")
				expectedValues := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
					"image": map[string]interface{}{
						"repository": "rancher-test.io/rancher/rancher-webhook",
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.1",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
			registryOverride: "rancher-test.io",
		},
		{
			name: "installation with min version override",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(emptyConfig, nil).Times(2)
				settings.RancherWebhookMinVersion.Set("2.0.1")
				settings.RancherWebhookVersion.Set("2.0.4")
				expectedValues := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": "",
						},
					},
					"image": map[string]interface{}{
						"repository": "rancher-test.io/rancher/rancher-webhook",
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"2.0.1",
					"",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"rancher-test.io/"+settings.ShellImage.Get(),
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
			registryOverride: "rancher-test.io",
		},
		{
			name: "installation with webhook values",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(fullConfig, nil).Times(2)
				settings.RancherWebhookVersion.Set("2.0.0")
				expectedValues := map[string]interface{}{
					"priorityClassName": priorityClassName,
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[any]interface{}{
						"enabled": false,
					},
					"global": "",
					"newKey": "newValue",
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
		},
		{
			name: "installation with invalid webhook values",
			setup: func(mocks testMocks) {
				mocks.namespaceCtrl.EXPECT().Delete(operatorNamespace, nil).Return(nil)
				mocks.configCache.EXPECT().Get(gomock.Any(), chart.CustomValueMapName).Return(invalidConfig, nil).Times(2)
				settings.RancherWebhookVersion.Set("2.0.0")
				expectedValues := map[string]interface{}{
					"capi": map[string]interface{}{
						"enabled": features.EmbeddedClusterAPI.Enabled(),
					},
					"mcm": map[string]interface{}{
						"enabled": features.MCM.Enabled(),
					},
					"global": map[string]interface{}{
						"cattle": map[string]interface{}{
							"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
						},
					},
				}
				mocks.manager.EXPECT().Ensure(
					namespace.System,
					"rancher-webhook",
					"",
					"2.0.0",
					expectedValues,
					gomock.AssignableToTypeOf(false),
					"",
				).Return(nil)

				mocks.manager.EXPECT().Uninstall(operatorNamespace, "rancher-operator").Return(nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset setting to default values before each test
			settings.RancherWebhookMinVersion.Set(originalMinVersion)
			settings.RancherWebhookVersion.Set(originalVersion)
			ctrl := gomock.NewController(t)

			// create mocks for each test
			mocks := testMocks{
				manager:       chartfake.NewMockManager(ctrl),
				namespaceCtrl: fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl),
				configCache:   fake.NewMockCacheInterface[*v1.ConfigMap](ctrl),
			}

			// allow test to add expected calls to mocks and run any additional setup
			tt.setup(mocks)
			h := mocks.Handler()

			// add any registryOverrides
			h.registryOverride = tt.registryOverride
			_, err := h.onRepo("", repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("handler.onRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
