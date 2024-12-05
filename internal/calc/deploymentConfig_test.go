package calc

import (
	"testing"

	openshiftAppsV1 "github.com/openshift/api/apps/v1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDeploymentConfig(t *testing.T) {
	var tests = []struct {
		name             string
		deploymentConfig string
		cpuMin           resource.Quantity
		cpuMax           resource.Quantity
		memoryMin        resource.Quantity
		memoryMax        resource.Quantity
		replicas         int32
		maxReplicas      int32
		strategy         openshiftAppsV1.DeploymentStrategyType
	}{
		{
			name:             "normal deploymentConfig",
			deploymentConfig: normalDeploymentConfig,
			cpuMin:           resource.MustParse("3250m"),
			cpuMax:           resource.MustParse("6500m"),
			memoryMin:        resource.MustParse("26Gi"),
			memoryMax:        resource.MustParse("52Gi"),
			replicas:         10,
			maxReplicas:      13,
			strategy:         openshiftAppsV1.DeploymentStrategyTypeRolling,
		},
		//TODO add more tests
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			resourceObject, kind, version, _ := ConvertToRuntimeObjectFromYaml([]byte(test.deploymentConfig), false)

			usage, err := ResourceQuotaFromYaml(ResourceObject{resourceObject, *kind, *version, nil})
			r.NoError(err)
			r.NotEmpty(usage)

			AssertEqualQuantities(r, test.cpuMin, usage.RolloutResources.CPUMin, "cpu request value")
			AssertEqualQuantities(r, test.cpuMax, usage.RolloutResources.CPUMax, "cpu limit value")
			AssertEqualQuantities(r, test.memoryMin, usage.RolloutResources.MemoryMin, "memory request value")
			AssertEqualQuantities(r, test.memoryMax, usage.RolloutResources.MemoryMax, "memory limit value")
			r.Equal(test.replicas, usage.Details.Replicas, "replicas")
			r.Equal(string(test.strategy), usage.Details.Strategy, "strategy")
			r.Equal(test.maxReplicas, usage.Details.MaxReplicas, "maxReplicas")
		})
	}
}
