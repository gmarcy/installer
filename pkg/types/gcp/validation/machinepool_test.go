package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types/gcp"
)

func TestValidateMachinePool(t *testing.T) {
	platform := &gcp.Platform{Region: "us-east1"}
	cases := []struct {
		name     string
		pool     *gcp.MachinePool
		expected string
	}{
		{
			name: "empty",
			pool: &gcp.MachinePool{},
		},
		{
			name: "valid zone",
			pool: &gcp.MachinePool{
				Zones: []string{"us-east1-b", "us-east1-c"},
			},
		},
		{
			name: "invalid zone",
			pool: &gcp.MachinePool{
				Zones: []string{"us-east1-b", "us-central1-f"},
			},
			expected: `^test-path\.zones\[1]: Invalid value: "us-central1-f": Zone not in configured region \(us-east1\)$`,
		},
		{
			name: "valid disk type",
			pool: &gcp.MachinePool{
				OSDisk: gcp.OSDisk{
					DiskType: "pd-standard",
				},
			},
		},
		{
			name: "invalid disk type",
			pool: &gcp.MachinePool{
				OSDisk: gcp.OSDisk{
					DiskType: "pd-",
				},
			},
			expected: `^test-path\.diskType: Unsupported value: "pd-": supported values: "pd-ssd", "pd-standard"$`,
		},
		{
			name: "valid disk size",
			pool: &gcp.MachinePool{
				OSDisk: gcp.OSDisk{
					DiskSizeGB: 100,
				},
			},
		},
		{
			name: "invalid disk type",
			pool: &gcp.MachinePool{
				OSDisk: gcp.OSDisk{
					DiskSizeGB: -120,
				},
			},
			expected: `^test-path\.diskSizeGB: Invalid value: -120: must be a positive value$`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMachinePool(platform, tc.pool, field.NewPath("test-path")).ToAggregate()
			if tc.expected == "" {
				assert.NoError(t, err)
			} else {
				assert.Regexp(t, tc.expected, err)
			}
		})
	}
}
