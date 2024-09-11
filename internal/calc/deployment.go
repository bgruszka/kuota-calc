package calc

import (
	"fmt"
	"math"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// calculates the cpu/memory resources a single deployment needs. Replicas and the deployment
// strategy are taken into account.
func deployment(deployment appsv1.Deployment) (*ResourceUsage, error) {
	var (
		resourceOverhead float64 // max overhead compute resources (percent)
		podOverhead      int32   // max overhead pods during deployment
	)

	replicas := deployment.Spec.Replicas
	strategy := deployment.Spec.Strategy

	if *replicas == 0 {
		return &ResourceUsage{
			CpuMin:    new(resource.Quantity),
			CpuMax:    new(resource.Quantity),
			MemoryMin: new(resource.Quantity),
			MemoryMax: new(resource.Quantity),
			Details: Details{
				Version:     deployment.APIVersion,
				Kind:        deployment.Kind,
				Name:        deployment.Name,
				Replicas:    *replicas,
				MaxReplicas: *replicas,
				Strategy:    string(strategy.Type),
			},
		}, nil
	}

	switch strategy.Type {
	case appsv1.RecreateDeploymentStrategyType:
		// no overhead on recreate
		resourceOverhead = 1
		podOverhead = 0
	case "":
		// RollingUpdate is the default an can be an empty string. If so, set the defaults
		// (https://pkg.go.dev/k8s.io/api/apps/v1?tab=doc#RollingUpdateDeployment) and continue calculation.
		defaults := intstr.FromString("25%")
		strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: &defaults,
				MaxSurge:       &defaults,
			},
		}

		fallthrough
	case appsv1.RollingUpdateDeploymentStrategyType:
		// Documentation: https://pkg.go.dev/k8s.io/api/apps/v1?tab=doc#RollingUpdateDeployment
		// all default values are set as stated in the docs
		var (
			maxUnavailableValue intstr.IntOrString
			maxSurgeValue       intstr.IntOrString
		)

		// can be nil, if so apply default value
		if strategy.RollingUpdate == nil {
			maxUnavailableValue = intstr.FromString("25%")
			maxSurgeValue = intstr.FromString("25%")
		} else {
			maxUnavailableValue = *strategy.RollingUpdate.MaxUnavailable
			maxSurgeValue = *strategy.RollingUpdate.MaxSurge
		}

		// docs say, that the asolute number is calculated by rounding down.
		maxUnavailable, err := intstr.GetValueFromIntOrPercent(&maxUnavailableValue, int(*replicas), false)
		if err != nil {
			return nil, err
		}

		// docs say, absolute number is calculated by rounding up.
		maxSurge, err := intstr.GetValueFromIntOrPercent(&maxSurgeValue, int(*replicas), true)
		if err != nil {
			return nil, err
		}

		// podOverhead is the number of pods which can run more during a deployment
		podOverhead = int32(maxSurge - maxUnavailable)

		resourceOverhead = (float64(podOverhead) / float64(*replicas)) + 1
	default:
		return nil, fmt.Errorf("deployment: %s deployment strategy %q is unknown", deployment.Name, strategy.Type)
	}

	cpuMin, cpuMax, memoryMin, memoryMax := podResources(&deployment.Spec.Template.Spec)

	memMin := float64(memoryMin.Value()) * float64(*replicas) * resourceOverhead
	memoryMin.Set(int64(math.Round(memMin)))

	memMax := float64(memoryMax.Value()) * float64(*replicas) * resourceOverhead
	memoryMax.Set(int64(math.Round(memMax)))

	cpuMin.SetMilli(int64(math.Round(float64(cpuMin.MilliValue()) * float64(*replicas) * resourceOverhead)))

	cpuMax.SetMilli(int64(math.Round(float64(cpuMax.MilliValue()) * float64(*replicas) * resourceOverhead)))

	resourceUsage := ResourceUsage{
		CpuMin:    cpuMin,
		CpuMax:    cpuMax,
		MemoryMin: memoryMin,
		MemoryMax: memoryMax,
		Details: Details{
			Version:     deployment.APIVersion,
			Kind:        deployment.Kind,
			Name:        deployment.Name,
			Replicas:    *replicas,
			Strategy:    string(strategy.Type),
			MaxReplicas: *replicas + podOverhead,
		},
	}

	return &resourceUsage, nil
}
