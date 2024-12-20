package calc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCronJob(t *testing.T) {
	var tests = []struct {
		name        string
		cronjob     string
		cpuMin      resource.Quantity
		cpuMax      resource.Quantity
		memoryMin   resource.Quantity
		memoryMax   resource.Quantity
		replicas    int32
		maxReplicas int32
		strategy    string
	}{
		{
			name:      "ok",
			cronjob:   normalCronJob,
			cpuMin:    resource.MustParse("250m"),
			cpuMax:    resource.MustParse("1"),
			memoryMin: resource.MustParse("2Gi"),
			memoryMax: resource.MustParse("4Gi"),
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				r := require.New(t)

				resourceObject, kind, version, _ := ConvertToRuntimeObjectFromYaml([]byte(test.cronjob), false)

				usage, err := ResourceQuotaFromYaml(ResourceObject{resourceObject, *kind, *version, nil})
				r.NoError(err)
				r.NotEmpty(usage)

				AssertEqualQuantities(r, test.cpuMin, usage.RolloutResources.CPUMin, "cpu request value")
				AssertEqualQuantities(r, test.cpuMax, usage.RolloutResources.CPUMax, "cpu limit value")
				AssertEqualQuantities(r, test.memoryMin, usage.RolloutResources.MemoryMin, "memory request value")
				AssertEqualQuantities(r, test.memoryMax, usage.RolloutResources.MemoryMax, "memory limit value")
				r.Equalf(test.replicas, usage.Details.Replicas, "replicas")
				r.Equalf(test.maxReplicas, usage.Details.MaxReplicas, "maxReplicas")
				r.Equalf(test.strategy, usage.Details.Strategy, "strategy")
			},
		)
	}
}
