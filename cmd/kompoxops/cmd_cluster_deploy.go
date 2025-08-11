package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/cluster"
	"github.com/yaegashi/kompoxops/models/cfgops"
)

// newCmdClusterDeploy installs cluster-level components like ingress controller.
func newCmdClusterDeploy() *cobra.Command {
	var file string
	var controller string
	var namespace string
	var wait bool
	var traefikChartVersion string
	var traefikExtraValues []string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy cluster add-ons (e.g., ingress controller)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				file = cfgops.DefaultConfigPath
			}
			cfg, err := cfgops.Load(file)
			if err != nil {
				return err
			}

			// Merge flags with config defaults
			if controller == "" {
				controller = cfg.Cluster.Ingress.Controller
			}
			if namespace == "" {
				namespace = cfg.Cluster.Ingress.Namespace
			}
			if controller == "" {
				controller = "traefik"
			}
			if namespace == "" {
				namespace = "traefik"
			}

			// Ensure cluster connectivity early (also picks kubeconfig/context)
			if _, err := cluster.New(cfg); err != nil {
				return err
			}

			switch strings.ToLower(controller) {
			case "traefik":
				return installTraefik(cmd, namespace, wait, traefikChartVersion, traefikExtraValues)
			default:
				return fmt.Errorf("unsupported ingress controller: %s", controller)
			}
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", cfgops.DefaultConfigPath, "Path to kompoxops.yml")
	cmd.Flags().StringVar(&controller, "ingress-controller", "", "Ingress controller to install (default from config; supports: traefik)")
	cmd.Flags().StringVar(&namespace, "ingress-namespace", "", "Namespace to install ingress controller in (default from config or 'traefik')")
	cmd.Flags().BoolVar(&wait, "wait", true, "Wait for resources to become ready")
	cmd.Flags().StringVar(&traefikChartVersion, "traefik-chart-version", "", "Specific Traefik Helm chart version to install (optional)")
	cmd.Flags().StringArrayVar(&traefikExtraValues, "traefik-set", nil, "Extra --set pairs for Traefik Helm install (key=value). Can be repeated.")
	return cmd
}

// installTraefik installs Traefik via Helm in the given namespace.
func installTraefik(cmd *cobra.Command, namespace string, wait bool, chartVersion string, extraSet []string) error {
	// Validate helm/kubectl availability
	if _, err := exec.LookPath("helm"); err != nil {
		return errors.New("helm not found in PATH; please install Helm")
	}
	if _, err := exec.LookPath("kubectl"); err != nil {
		return errors.New("kubectl not found in PATH; please install kubectl")
	}

	// 1) Add/update repo
	if err := runCmd(cmd, []string{"helm", "repo", "add", "traefik", "https://traefik.github.io/charts"}); err != nil {
		return err
	}
	if err := runCmd(cmd, []string{"helm", "repo", "update"}); err != nil {
		return err
	}

	// 2) Create namespace if not exists
	_ = runCmd(cmd, []string{"kubectl", "get", "ns", namespace})
	_ = runCmd(cmd, []string{"kubectl", "create", "ns", namespace})

	// 3) Prepare args and install/upgrade
	args := []string{"helm", "upgrade", "--install", "traefik", "traefik/traefik", "-n", namespace}
	if wait {
		args = append(args, "--wait")
		// Reasonable timeout
		args = append(args, "--timeout", "10m")
	}
	if chartVersion != "" {
		args = append(args, "--version", chartVersion)
	}
	// Defaults suitable for many clusters: enable ingressClass=traefik
	defaults := []string{
		"ingressClass.enabled=true",
		"ingressClass.isDefaultClass=true",
		"service.enabled=true",
		"service.type=LoadBalancer",
		// Useful metrics can be enabled later; keep minimal by default
	}
	for _, kv := range append(defaults, extraSet...) {
		if strings.TrimSpace(kv) == "" {
			continue
		}
		args = append(args, "--set", kv)
	}

	if err := runCmd(cmd, args); err != nil {
		return err
	}

	// 4) Optional readiness check for service/traefik loadbalancer
	if wait {
		// best-effort check; ignore errors after timeout
		_ = waitForLoadBalancerIP(cmd, namespace, "traefik")
	}
	return nil
}

func runCmd(cmd *cobra.Command, args []string) error {
	c := exec.CommandContext(cmd.Context(), args[0], args[1:]...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

func waitForLoadBalancerIP(cmd *cobra.Command, namespace, svc string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// kubectl get svc -n <ns> <svc> -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
		out, err := exec.CommandContext(ctx, "kubectl", "get", "svc", "-n", namespace, svc, "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}").CombinedOutput()
		if err == nil {
			ip := strings.TrimSpace(string(out))
			if ip != "" && ip != "<no value>" {
				fmt.Fprintf(cmd.OutOrStdout(), "traefik LoadBalancer IP=%s\n", ip)
				return nil
			}
		}
		time.Sleep(5 * time.Second)
	}
}
