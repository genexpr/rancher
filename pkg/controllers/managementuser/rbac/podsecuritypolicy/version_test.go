package podsecuritypolicy

import (
	"fmt"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"
)

func newClusterListerWithVersion(kubernetesVersion string) *fakes.ClusterListerMock {
	return &fakes.ClusterListerMock{
		GetFunc: func(namespace, name string) (*mgmtv3.Cluster, error) {
			if name == "test" {
				cluster := mgmtv3.Cluster{
					Status: apimgmtv3.ClusterStatus{
						Version: &version.Info{
							GitVersion: kubernetesVersion,
						},
					},
				}
				return &cluster, nil
			}
			return nil, fmt.Errorf("invalid cluster: %s", name)
		},
	}
}

func TestCheckClusterVersion(t *testing.T) {
	t.Parallel()
	tests := []*struct {
		version string
		wantErr bool
		setup   func()
	}{
		// tests for version string size in characters
		{
			version: "",
			wantErr: true,
		},
		{
			version: "⌘⌘⌘",
			wantErr: true,
		},
		{
			version: "v1.2",
			wantErr: true,
		},
		{
			version: "v1.24",
			wantErr: true,
		},
		{
			version: "v1.24.9",
			wantErr: false,
		},
		{
			version: "v1.25.9",
			wantErr: true,
		},
		{
			version: "v1.26.9",
			wantErr: true,
		},
		// k3s version strings
		{
			version: "v1.24.9+k3s1",
			wantErr: false,
		},
		{
			version: "v1.25.9+k3s1",
			wantErr: true,
		},
		{
			version: "v1.26.9+k3s1",
			wantErr: true,
		},
		// rke2 version strings
		{
			version: "v1.24.9+rke2r1",
			wantErr: false,
		},
		{
			version: "v1.25.9+rke2r1",
			wantErr: true,
		},
		{
			version: "v1.26.9+rke2r1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()
			clusterLister := newClusterListerWithVersion(tt.version)
			err := checkClusterVersion("test", clusterLister)
			if tt.wantErr {
				require.Error(t, err)
			}
			if !tt.wantErr {
				require.NoError(t, err)
			}
		})
	}

	t.Run("version check fails when it can't get cluster", func(t *testing.T) {
		t.Parallel()
		clusterLister := newClusterListerWithVersion("bad")
		err := checkClusterVersion("test", clusterLister)
		require.Error(t, err)
	})
}
