package calc

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestPod(t *testing.T) {
	var tests = []struct {
		name        string
		pod         string
		cpuMin      resource.Quantity
		cpuMax      resource.Quantity
		memoryMin   resource.Quantity
		memoryMax   resource.Quantity
		replicas    int32
		maxReplicas int32
		strategy    appsv1.StatefulSetUpdateStrategyType
	}{
		{
			name:      "normal pod",
			pod:       normalPod,
			cpuMin:    resource.MustParse("250m"),
			cpuMax:    resource.MustParse("1"),
			memoryMin: resource.MustParse("2Gi"),
			memoryMax: resource.MustParse("4Gi"),
		},
		{
			name:      "pod with multiple containers",
			pod:       multiContainerPod,
			cpuMin:    resource.MustParse("400m"),
			cpuMax:    resource.MustParse("1750m"),
			memoryMin: resource.MustParse("3Gi"),
			memoryMax: resource.MustParse("7Gi"),
		},
		{
			name:      "pod with small init container",
			pod:       initContainerPod,
			cpuMin:    resource.MustParse("250m"),
			cpuMax:    resource.MustParse("1"),
			memoryMin: resource.MustParse("2Gi"),
			memoryMax: resource.MustParse("4Gi"),
		},
		{
			name:      "pod with big init container",
			pod:       bigInitContainerPod,
			cpuMin:    resource.MustParse("1"),
			cpuMax:    resource.MustParse("2"),
			memoryMin: resource.MustParse("3Gi"),
			memoryMax: resource.MustParse("5Gi"),
		},
		{
			// This testcase is for taking the max of init and normal containers for each resource
			name:      "pod with a similar sized init container to the normal containers",
			pod:       mediumInitContainerPod,
			cpuMin:    resource.MustParse("250m"),
			cpuMax:    resource.MustParse("2"),
			memoryMin: resource.MustParse("3Gi"),
			memoryMax: resource.MustParse("4Gi"),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				r := require.New(t)

				resourceObject, kind, version, _ := ConvertToRuntimeObjectFromYaml([]byte(test.pod), false)

				usage, err := ResourceQuotaFromYaml(ResourceObject{resourceObject, *kind, *version, nil})
				r.NoError(err)
				r.NotEmpty(usage)

				AssertEqualQuantities(r, test.cpuMin, usage.RolloutResources.CPUMin, "cpu request value")
				AssertEqualQuantities(r, test.cpuMax, usage.RolloutResources.CPUMax, "cpu limit value")
				AssertEqualQuantities(r, test.memoryMin, usage.RolloutResources.MemoryMin, "memory request value")
				AssertEqualQuantities(r, test.memoryMax, usage.RolloutResources.MemoryMax, "memory limit value")
				r.Equalf(test.replicas, usage.Details.Replicas, "replicas")
				r.Equalf(test.maxReplicas, usage.Details.MaxReplicas, "maxReplicas")
				r.Equalf(string(test.strategy), usage.Details.Strategy, "strategy")
			},
		)
	}
}
