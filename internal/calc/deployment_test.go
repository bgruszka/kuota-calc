package calc

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDeployment(t *testing.T) {
	var tests = []struct {
		name        string
		deployment  string
		cpuMin      resource.Quantity
		cpuMax      resource.Quantity
		memoryMin   resource.Quantity
		memoryMax   resource.Quantity
		replicas    int32
		maxReplicas int32
		strategy    appsv1.DeploymentStrategyType
	}{
		{
			// replicas=10, maxSurge=25%=3, maxUnavailable=25%=2.
			// Rollout reserves: replicas + maxSurge + maxUnavailable = 15 pods worth of resources,
			// where (replicas - maxUnavailable + terminatingPodCount) = 10 pods use Containers and
			// (maxSurge + maxUnavailable) = 5 pods use MaxResources (== Containers, no init container).
			name:        "normal deployment",
			deployment:  normalDeployment,
			cpuMin:      resource.MustParse("3750m"),
			cpuMax:      resource.MustParse("7500m"),
			memoryMin:   resource.MustParse("30Gi"),
			memoryMax:   resource.MustParse("60Gi"),
			replicas:    10,
			maxReplicas: 13,
			strategy:    appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			name:        "deployment without strategy",
			deployment:  deploymentWithoutStrategy,
			cpuMin:      resource.MustParse("3750m"),
			cpuMax:      resource.MustParse("15"),
			memoryMin:   resource.MustParse("30Gi"),
			memoryMax:   resource.MustParse("60Gi"),
			replicas:    10,
			maxReplicas: 13,
			strategy:    appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			name:        "deployment with absolute unavailable/surge values",
			deployment:  deploymentWithAbsoluteValues,
			cpuMin:      resource.MustParse("3"),
			cpuMax:      resource.MustParse("12"),
			memoryMin:   resource.MustParse("24Gi"),
			memoryMax:   resource.MustParse("48Gi"),
			replicas:    10,
			maxReplicas: 12,
			strategy:    appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			name:        "zero replica deployment",
			deployment:  zeroReplicaDeployment,
			cpuMin:      resource.MustParse("0"),
			cpuMax:      resource.MustParse("0"),
			memoryMin:   resource.MustParse("0"),
			memoryMax:   resource.MustParse("0"),
			replicas:    0,
			maxReplicas: 0,
			strategy:    appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			name:        "recreate deployment",
			deployment:  recrateDeployment,
			cpuMin:      resource.MustParse("2500m"),
			cpuMax:      resource.MustParse("10"),
			memoryMin:   resource.MustParse("20Gi"),
			memoryMax:   resource.MustParse("40Gi"),
			replicas:    10,
			maxReplicas: 10,
			strategy:    appsv1.RecreateDeploymentStrategyType,
		},
		{
			name:        "deployment without max unavailable/surge values",
			deployment:  deploymentWithoutValues,
			cpuMin:      resource.MustParse("3750m"),
			cpuMax:      resource.MustParse("15"),
			memoryMin:   resource.MustParse("30Gi"),
			memoryMax:   resource.MustParse("60Gi"),
			replicas:    10,
			maxReplicas: 13,
			strategy:    appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			// replicas=10, maxSurge=2, maxUnavailable=2. The naive formula
			// (replicas - maxUnavailable) * Containers + (maxSurge + maxUnavailable) * MaxResources
			// would reserve only 12 pods worth of resources; the corrected formula reserves 14
			// to cover the 2 pods that may still be in Terminating state while the controller
			// has already scheduled their replacements.
			name:        "rolling update reserves capacity for terminating pods",
			deployment:  rollingUpdateWithTerminatingOverlap,
			cpuMin:      resource.MustParse("3500m"),
			cpuMax:      resource.MustParse("14"),
			memoryMin:   resource.MustParse("28Gi"),
			memoryMax:   resource.MustParse("56Gi"),
			replicas:    10,
			maxReplicas: 12,
			strategy:    appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			name:        "deployment with init container(s)",
			deployment:  initContainerDeployment,
			cpuMin:      resource.MustParse("1000m"),
			cpuMax:      resource.MustParse("4000m"),
			memoryMin:   resource.MustParse("8Gi"),
			memoryMax:   resource.MustParse("16Gi"),
			replicas:    3,
			maxReplicas: 4,
			strategy:    appsv1.RollingUpdateDeploymentStrategyType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			resourceObject, kind, version, _ := ConvertToRuntimeObjectFromYaml([]byte(test.deployment), false)

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
