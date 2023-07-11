package chart_test

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var errTest = fmt.Errorf("test error")

const priorityClassName = "rancher-critical"

func TestGetPriorityClassNameFromRancherConfigMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	configCache := fake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
	configCache.EXPECT().Get(namespace.System, "set-config").Return(&v1.ConfigMap{Data: map[string]string{"priorityClassName": priorityClassName}}, nil).AnyTimes()
	configCache.EXPECT().Get(namespace.System, "empty-config").Return(&v1.ConfigMap{}, nil).AnyTimes()
	configCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("not found")).AnyTimes()

	tests := []*struct {
		name    string
		want    string
		wantErr bool
		setup   func()
	}{
		// base case config map set.
		{
			name:    "correctly set priority class name",
			want:    priorityClassName,
			wantErr: false,
			setup:   func() { settings.ConfigMapName.Set("set-config") },
		},
		// config map name is empty.
		{
			name:    "empty configMap name",
			want:    "",
			wantErr: true,
			setup:   func() { settings.ConfigMapName.Set("") },
		},
		// config map doesn't exist.
		{
			name:    "unknown config map name",
			want:    "",
			wantErr: true,
			setup:   func() { settings.ConfigMapName.Set("unknown-config-name") },
		},
		// config map exist doesn't have priority class.
		{
			name:    "empty config map",
			want:    "",
			wantErr: true,
			setup:   func() { settings.ConfigMapName.Set("empty-config") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			getter := chart.RancherConfigGetter{configCache}
			got, err := getter.GetPriorityClassName()
			if tt.wantErr {
				assert.Error(t, err, "Expected test to error.")
				return
			}
			assert.NoError(t, err, "failed to get priority class.")
			assert.Equal(t, tt.want, got, "Unexpected priorityClassName returned")
		})
	}
}

func TestGetWebhookValue(t *testing.T) {
	const yamlInfo = "yamlKey: yamlValue"
	const invalidYaml = "%{'foo':'bar "

	tests := []*struct {
		name     string
		want     map[string]any
		wantErr  bool
		notFound bool
		setup    func(*fake.MockCacheInterface[*v1.ConfigMap])
	}{
		{
			name: "correctly get webhook values",
			want: map[string]any{"yamlKey": "yamlValue"},
			setup: func(configCache *fake.MockCacheInterface[*v1.ConfigMap]) {
				configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(&v1.ConfigMap{Data: map[string]string{chart.WebhookChartName: yamlInfo}}, nil)
			},
		},

		{
			name:    "webhook values are invalid yaml",
			wantErr: true,
			setup: func(configCache *fake.MockCacheInterface[*v1.ConfigMap]) {
				configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(&v1.ConfigMap{Data: map[string]string{chart.WebhookChartName: invalidYaml}}, nil)
			},
		},
		{
			name:     "value not in map",
			wantErr:  true,
			notFound: true,
			setup: func(configCache *fake.MockCacheInterface[*v1.ConfigMap]) {
				configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(&v1.ConfigMap{Data: map[string]string{}}, nil)
			},
		},
		{
			name:     "map is nil",
			wantErr:  true,
			notFound: true,
			setup: func(configCache *fake.MockCacheInterface[*v1.ConfigMap]) {
				configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(&v1.ConfigMap{Data: nil}, nil)
			},
		},
		{
			name:     "rancher config does not exist",
			wantErr:  true,
			notFound: true,
			setup: func(configCache *fake.MockCacheInterface[*v1.ConfigMap]) {
				configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(nil, apierror.NewNotFound(schema.GroupResource{}, chart.CustomValueMapName))
			},
		},
		{
			name:     "rancher config get failed",
			wantErr:  true,
			notFound: false,
			setup: func(configCache *fake.MockCacheInterface[*v1.ConfigMap]) {
				configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(nil, errTest)
			},
		},
		{
			name: "webhook value does not used deprecated configMap setting",
			want: map[string]any{"yamlKey": "yamlValue"},
			setup: func(configCache *fake.MockCacheInterface[*v1.ConfigMap]) {
				settings.ConfigMapName.Set("unknown-config-name")
				configCache.EXPECT().Get(namespace.System, chart.CustomValueMapName).Return(&v1.ConfigMap{Data: map[string]string{chart.WebhookChartName: yamlInfo}}, nil)
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			configCache := fake.NewMockCacheInterface[*v1.ConfigMap](ctrl)
			test.setup(configCache)
			getter := chart.RancherConfigGetter{configCache}
			got, err := getter.GetWebhookValues()
			if test.wantErr {
				assert.Equal(t, test.notFound, chart.IsNotFoundError(err))
				assert.Error(t, err, "Expected test to error.")
				return
			}
			assert.NoError(t, err, "failed to get priority class.")
			assert.Equal(t, test.want, got, "Unexpected priorityClassName returned")
		})
	}
}
