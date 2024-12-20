![ci](https://github.com/bgruszka/kuota-calc/workflows/ci/badge.svg)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/bgruszka/kuota-calc)
[![Go Report Card](https://goreportcard.com/badge/github.com/bgruszka/kuota-calc)](https://goreportcard.com/report/github.com/bgruszka/kuota-calc)
![License](https://img.shields.io/github/license/bgruszka/kuota-calc)

> [!NOTE]
> This is a fork of [druppelt/kuota-calc](https://github.com/druppelt/kuota-calc) that adds some features:
> * added support for HorizontalPodAutoscaler
> * added JSON output format
> * added suppression of warning about unsupported kinds

# kuota-calc
Simple utility to calculate the maximum needed resource quota for deployment(s). kuota-calc takes the
deployment strategy, replicas and all containers into account, see [supported-resources](https://github.com/bgruszka/kuota-calc#supported-k8s-resources) for a list of kubernetes resources which are currently supported by kuota-calc.

## Motivation
In shared environments such as kubernetes it is always a good idea to isolate/constrain different workloads to prevent them from interfering each other. Kubernetes provides [Resource Quotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/) to limit compute, storage and object resources of namespaces.

Calculating the needed compute resources can be a bit challenging (especially with large and complex deployments) because we must respect certain settings/defaults like the deployment strategy, number of replicas and so on. This is where kuota-calc can help you, it calculates the maximum needed resource quota in order to be able to start a deployment of all resources at the same time by respecting deployment strategies, replicas and so on.

## Example
Get a detailed report of all resources, their max required quota and a total. 
```bash
$ cat examples/deployment.yaml | kuota-calc -detailed
Version    Kind           Name     Replicas    Strategy         MaxReplicas    CPURequest    CPULimit    MemoryRequest    MemoryLimit    
apps/v1    Deployment     myapp    10          RollingUpdate    13             3250m         6500m       832Mi            3328Mi         
apps/v1    StatefulSet    myapp    3           RollingUpdate    3              750m          3           6Gi              12Gi           

Table and Total assuming simultaneous rollout of all resources

Total
CPU Request: 4
CPU Limit: 9500m
Memory Request: 6976Mi
Memory Limit: 15616Mi
```

For comparison, here the simultaneous rollout is limited to zero resources, so you get the required quotas to just run, but not deploy the applications. 
````bash
$ cat examples/deployment.yaml | kuota-calc --max-rollouts=0
CPU Request: 3250m
CPU Limit: 8
Memory Request: 6784Mi
Memory Limit: 14848Mi
````

To calc usage for deploymentConfigs, deployments and statefulSets deployed in an openshift cluster:
```bash
$ oc get dc,sts,deploy -o json | yq -p=json -o=yaml '.items[] | split_doc' | kuota-calc --detailed
Warning: apps.openshift.io/v1 DeploymentConfig is deprecated in v4.14+, unavailable in v4.10000+
Version                 Kind                Name                        Replicas    Strategy         MaxReplicas    CPURequest    CPULimit    MemoryRequest    MemoryLimit
apps.openshift.io/v1    DeploymentConfig    my-app-1                    0           Recreate         0              0             0           0                0
apps.openshift.io/v1    DeploymentConfig    my-app-2                    1           Recreate         1              1250m         1700m       500Mi            500Mi
apps/v1                 StatefulSet         my-app-3                    1           RollingUpdate    1              150m          1100m       2200Mi           2200Mi
apps/v1                 Deployment          my-app-4                    1           RollingUpdate    2              100m          200m        100Mi            512Mi

Total
CPU Request: 1500m
CPU Limit: 3
Memory Request: 2800Mi
Memory Limit: 3212Mi
```

## Installation
Pre-compiled statically linked binaries are available on the [releases page](https://github.com/bgruszka/kuota-calc/releases).

kuota-calc can either be used as a kubectl plugin or invoked directly. If you intend to use kuota-calc as
a kubectl plugin, simply place the binary anywhere in `$PATH` named `kubectl-kuota_calc` with execute permissions.
For further information, see the official documentation on kubectl plugins [here](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/).

**currently the kubectl plugin is not released for this fork**

## supported k8s and os resources
**kuota-calc is still a work-in progress**, there are plans to support more k8s resources (see [#5](https://github.com/postfinance/kuota-calc/issues/5) for more info). 

Currently supported:

- apps.openshift.io/v1 DeploymentConfig
- apps/v1 Deployment
- apps/v1 StatefulSet
- apps/v1 DaemonSet
- batch/v1 CronJob
- batch/v1 Job
- v1 Pod
- autoscaling/v2 HorizontalPodAutoscaler

## known limitation
- CronJobs: the cron concurrencyPolicy is not considered, a CronJob is treated as a single Pod (#18)
- DaemonSet: neither node count nor UpdateStrategy are considered. Treated as a single Pod. (#21)