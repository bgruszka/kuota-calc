// Package cmd provides the kuota-calc command.
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/util/json"
	"log"
	"runtime"
	"text/tabwriter"

	"github.com/druppelt/kuota-calc/internal/calc"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	kuotaCalcExample = `    # provide a simple/complex deployment by piping it to kuota-calc (used as kubectl plugin)
    cat deployment.yaml | kubectl %[1]s

    # do the same, calling the binary directly with detailed output
    cat deployment.yaml | %[1]s --detailed`
)

type JsonResource struct {
	Version       string `json:"version"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Replicas      int32  `json:"replicas"`
	Strategy      string `json:"strategy"`
	MaxReplicas   int32  `json:"maxReplicas"`
	CPURequest    string `json:"CPURequest"`
	CPULimit      string `json:"CPULimit"`
	MemoryRequest string `json:"memoryRequest"`
	MemoryLimit   string `json:"memoryLimit"`
	IsHPA         bool   `json:"isHPA"`
}

type JsonOutput struct {
	Resources []JsonResource `json:"resources"`
}

// KuotaCalcOpts holds all command options.
type KuotaCalcOpts struct {
	genericclioptions.IOStreams

	// flags
	debug                              bool
	detailed                           bool
	version                            bool
	maxRollouts                        int
	json                               bool
	suppressWarningForUnregisteredKind bool
	// files    []string

	versionInfo *Version
}

// NewKuotaCalcCmd returns a coba command wrapping KuotaCalcOps
func NewKuotaCalcCmd(version *Version, streams genericclioptions.IOStreams) *cobra.Command {
	opts := KuotaCalcOpts{
		IOStreams:   streams,
		versionInfo: version,
	}

	cmd := &cobra.Command{
		Use:          "kuota-calc",
		Short:        "Calculate the resource quota needs of your deployment(s).",
		Example:      fmt.Sprintf(kuotaCalcExample, "kuota-calc"),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if opts.version {
				return opts.printVersion()
			}

			return opts.run()
		},
	}

	cmd.Flags().BoolVar(&opts.debug, "debug", false, "enable debug logging")
	cmd.Flags().BoolVar(&opts.detailed, "detailed", false, "enable detailed output")
	cmd.Flags().BoolVar(&opts.version, "version", false, "print version and exit")
	cmd.Flags().IntVar(&opts.maxRollouts, "max-rollouts", -1, "limit the simultaneous rollout to the n most expensive rollouts per resource")
	cmd.Flags().BoolVar(&opts.json, "json", false, "output to json")
	cmd.Flags().BoolVar(&opts.suppressWarningForUnregisteredKind, "suppressWarningForUnregisteredKind", false, "suppress warning for unregistered kind")

	return cmd
}

func (opts *KuotaCalcOpts) printVersion() error {
	_, _ = fmt.Fprintf(opts.Out, "version %s (revision: %s)\n\tbuild date: %s\n\tgo version: %s\n",
		opts.versionInfo.Version,
		opts.versionInfo.Commit,
		opts.versionInfo.Date,
		runtime.Version(),
	)

	return nil
}

func (opts *KuotaCalcOpts) run() error {
	var (
		summary []*calc.ResourceUsage
	)

	yamlReader := yaml.NewYAMLReader(bufio.NewReader(opts.In))

	hpas := []*v2.HorizontalPodAutoscaler{}
	objects := []calc.ResourceObject{}

	for {
		data, err := yamlReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("reading input: %w", err)
		}

		runtimeObject, kind, version, err := calc.ConvertToRuntimeObjectFromYaml(data, opts.suppressWarningForUnregisteredKind)

		horizontalPodAutoscaler, ok := runtimeObject.(*v2.HorizontalPodAutoscaler)

		if ok {
			hpas = append(hpas, horizontalPodAutoscaler)
		}

		objects = append(objects, calc.ResourceObject{Object: runtimeObject, Kind: *kind, Version: *version})
	}

	for _, obj := range objects {
		deployment, ok := obj.Object.(*appsv1.Deployment)

		if ok {
			for _, hpa := range hpas {
				if hpa.Spec.ScaleTargetRef.Name == deployment.Name {
					obj.LinkedObject = hpa
				}
			}
		}

		usage, err := calc.ResourceQuotaFromYaml(obj)
		if err != nil {
			if errors.Is(err, calc.ErrResourceNotSupported) {
				if opts.debug {
					_, _ = fmt.Fprintf(opts.Out, "DEBUG: %s\n", err)
				}

				continue
			}

			return err
		}

		summary = append(summary, usage)
	}

	if opts.detailed {
		opts.printDetailed(summary)
	} else {
		opts.printSummary(summary)
	}

	return nil
}

func (opts *KuotaCalcOpts) printDetailed(usage []*calc.ResourceUsage) {
	if !opts.json {
		w := tabwriter.NewWriter(opts.Out, 0, 0, 4, ' ', tabwriter.TabIndent)

		_, _ = fmt.Fprintf(w, "Version\tKind\tName\tReplicas\tStrategy\tMaxReplicas\tCPURequest\tCPULimit\tMemoryRequest\tMemoryLimit\tIsHPA\t\n")

		for _, u := range usage {
			isHpa := "false"
			if u.Details.Hpa {
				isHpa = "true"
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t\n",
				u.Details.Version,
				u.Details.Kind,
				u.Details.Name,
				u.Details.Replicas,
				u.Details.Strategy,
				u.Details.MaxReplicas,
				u.RolloutResources.CPUMin.String(),
				u.RolloutResources.CPUMax.String(),
				u.RolloutResources.MemoryMin.String(),
				u.RolloutResources.MemoryMax.String(),
				isHpa,
			)
		}

		if err := w.Flush(); err != nil {
			_, _ = fmt.Fprintf(opts.Out, "printing detailed resources to tabwriter failed: %v\n", err)
		}

		if opts.maxRollouts > -1 {
			_, _ = fmt.Fprintf(opts.Out, "\nTable assuming simultaneous rollout of all resources\n")
			_, _ = fmt.Fprintf(opts.Out, "Total assuming simultaneous rollout of %d resources\n", opts.maxRollouts)
		} else {
			_, _ = fmt.Fprintf(opts.Out, "\nTable and Total assuming simultaneous rollout of all resources\n")
		}

		_, _ = fmt.Fprintf(opts.Out, "\nTotal\n")

		opts.printSummary(usage)
	} else {
		jsonOutput := JsonOutput{}
		jsonItems := []JsonResource{}

		for _, u := range usage {
			jsonItems = append(jsonItems, JsonResource{
				Version:       u.Details.Version,
				Kind:          u.Details.Kind,
				Name:          u.Details.Name,
				Replicas:      u.Details.Replicas,
				Strategy:      u.Details.Strategy,
				MaxReplicas:   u.Details.MaxReplicas,
				CPURequest:    u.RolloutResources.CPUMin.String(),
				CPULimit:      u.RolloutResources.CPUMax.String(),
				MemoryRequest: u.RolloutResources.MemoryMin.String(),
				MemoryLimit:   u.RolloutResources.MemoryMax.String(),
				IsHPA:         u.Details.Hpa,
			})
		}

		jsonOutput.Resources = jsonItems

		marshaled, err := json.Marshal(jsonOutput)

		if err != nil {
			log.Fatalf("marshaling error: %s", err)
		}

		_, _ = fmt.Fprintln(opts.Out, string(marshaled))
	}
}

func (opts *KuotaCalcOpts) printSummary(usage []*calc.ResourceUsage) {
	totalResources := calc.Total(opts.maxRollouts, usage)

	_, _ = fmt.Fprintf(opts.Out, "CPU Request: %s\nCPU Limit: %s\nMemory Request: %s\nMemory Limit: %s\n",
		totalResources.CPUMin.String(),
		totalResources.CPUMax.String(),
		totalResources.MemoryMin.String(),
		totalResources.MemoryMax.String(),
	)
}
