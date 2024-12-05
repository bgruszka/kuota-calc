package calc

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDaemonSet(t *testing.T) {
	var tests = []struct {
		name        string
		daemonset   string
		cpuMin      resource.Quantity
		cpuMax      resource.Quantity
		memoryMin   resource.Quantity
		memoryMax   resource.Quantity
		replicas    int32
		maxReplicas int32
		strategy    appsv1.StatefulSetUpdateStrategyType
	}{
		{
			name:        "ok",
			daemonset:   normalDaemonSet,
			replicas:    1,
			maxReplicas: 1,
			cpuMin:      resource.MustParse("500m"),
			cpuMax:      resource.MustParse("2"),
			memoryMin:   resource.MustParse("200Mi"),
			memoryMax:   resource.MustParse("2Gi"),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				r := require.New(t)

				resourceObject, kind, version, _ := ConvertToRuntimeObjectFromYaml([]byte(test.daemonset), false)

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
