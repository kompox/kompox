package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/models/cfgops"
	"github.com/yaegashi/kompoxops/resources/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newCmdClusterInfo shows the cluster information from kompoxops.yml
func newCmdClusterInfo() *cobra.Command {
	var file string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show cluster info",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				file = cfgops.DefaultConfigPath
			}
			cfg, err := cfgops.Load(file)
			if err != nil {
				return err
			}
			cl := cfg.Cluster

			// Compute DNS base domains
			baseDomain := ""
			if cl.Name != "" && cfg.Service.Domain != "" {
				baseDomain = fmt.Sprintf("%s.%s", cl.Name, cfg.Service.Domain)
			}
			wildcard := ""
			if baseDomain != "" {
				wildcard = fmt.Sprintf("*.%s", baseDomain)
			}

			// Suggest default app hosts when app name is present
			var appDefault80, appDefault8080 string
			if cfg.App.Name != "" && baseDomain != "" {
				appDefault80 = fmt.Sprintf("%s.%s", cfg.App.Name, baseDomain)
				appDefault8080 = fmt.Sprintf("%s-8080.%s", cfg.App.Name, baseDomain)
			}

			// Best-effort Traefik detection and LB IP/hostname retrieval
			traefikNS := cl.Ingress.Namespace
			if traefikNS == "" {
				traefikNS = "traefik"
			}
			traefikChecked := false
			traefikInstalled := false
			traefikLBIP := ""
			traefikLBHostname := ""
			// Try to connect to cluster and read service status
			if c, err := cluster.New(cfg); err == nil {
				ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
				defer cancel()
				// Attempt to get Service/traefik in configured namespace
				if svc, err := c.Client.CoreV1().Services(traefikNS).Get(ctx, "traefik", metav1.GetOptions{}); err == nil {
					traefikChecked = true
					traefikInstalled = true
					if len(svc.Status.LoadBalancer.Ingress) > 0 {
						ing := svc.Status.LoadBalancer.Ingress[0]
						traefikLBIP = ing.IP
						traefikLBHostname = ing.Hostname
					}
				} else {
					// Fallback: also try default namespace "traefik" if different
					if traefikNS != "traefik" {
						if svc, err2 := c.Client.CoreV1().Services("traefik").Get(ctx, "traefik", metav1.GetOptions{}); err2 == nil {
							traefikChecked = true
							traefikInstalled = true
							if len(svc.Status.LoadBalancer.Ingress) > 0 {
								ing := svc.Status.LoadBalancer.Ingress[0]
								traefikLBIP = ing.IP
								traefikLBHostname = ing.Hostname
							}
						}
					}
				}
			}

			// When --json is specified, output a structured JSON
			if asJSON {
				out := map[string]any{
					"service": map[string]any{
						"name":   cfg.Service.Name,
						"domain": cfg.Service.Domain,
					},
					"cluster": map[string]any{
						"name":     cl.Name,
						"provider": cl.Provider,
						"auth": map[string]any{
							"type":       cl.Auth.Type,
							"context":    cl.Auth.Context,
							"kubeconfig": cl.Auth.Kubeconfig,
						},
						"ingress": map[string]any{
							"controller": cl.Ingress.Controller,
							"namespace":  cl.Ingress.Namespace,
							"traefik": map[string]any{
								"checked":   traefikChecked,
								"installed": traefikInstalled,
								"service":   "traefik",
								"namespace": traefikNS,
								"loadBalancer": map[string]any{
									"ip":       traefikLBIP,
									"hostname": traefikLBHostname,
								},
							},
						},
					},
					"dns": map[string]any{
						"base":     baseDomain,
						"wildcard": wildcard,
					},
				}
				if appDefault80 != "" || appDefault8080 != "" || len(cfg.App.Ingress) > 0 {
					out["app"] = map[string]any{
						"name": cfg.App.Name,
						"hosts": map[string]any{
							"http_80":   appDefault80,
							"http_8080": appDefault8080,
							"custom":    cfg.App.Ingress,
						},
					}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			// Text output (key=value), multi-line for readability
			fmt.Fprintf(cmd.OutOrStdout(), "service.name=%s\n", cfg.Service.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "service.domain=%s\n", cfg.Service.Domain)
			fmt.Fprintf(cmd.OutOrStdout(), "cluster.name=%s\n", cl.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "cluster.provider=%s\n", cl.Provider)
			fmt.Fprintf(cmd.OutOrStdout(), "auth.type=%s\n", cl.Auth.Type)
			if cl.Auth.Context != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "auth.context=%s\n", cl.Auth.Context)
			}
			if cl.Auth.Kubeconfig != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "auth.kubeconfig=%s\n", cl.Auth.Kubeconfig)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ingress.controller=%s\n", cl.Ingress.Controller)
			fmt.Fprintf(cmd.OutOrStdout(), "ingress.namespace=%s\n", cl.Ingress.Namespace)
			if traefikChecked {
				fmt.Fprintf(cmd.OutOrStdout(), "ingress.traefik.installed=%t\n", traefikInstalled)
				fmt.Fprintf(cmd.OutOrStdout(), "ingress.traefik.service=traefik\n")
				fmt.Fprintf(cmd.OutOrStdout(), "ingress.traefik.namespace=%s\n", traefikNS)
				if traefikLBIP != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "ingress.traefik.lb.ip=%s\n", traefikLBIP)
				}
				if traefikLBHostname != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "ingress.traefik.lb.hostname=%s\n", traefikLBHostname)
				}
			}
			if baseDomain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "dns.base=%s\n", baseDomain)
				fmt.Fprintf(cmd.OutOrStdout(), "dns.wildcard=%s\n", wildcard)
			}
			if appDefault80 != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "app.host.http_80=%s\n", appDefault80)
			}
			if appDefault8080 != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "app.host.http_8080=%s\n", appDefault8080)
			}
			// Custom app ingress mappings, if any
			if len(cfg.App.Ingress) > 0 {
				for k, v := range cfg.App.Ingress {
					fmt.Fprintf(cmd.OutOrStdout(), "app.host.custom.%s=%s\n", k, v)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", cfgops.DefaultConfigPath, "Path to kompoxops.yml")
	cmd.Flags().BoolVarP(&asJSON, "json", "j", false, "Output as JSON")
	return cmd
}
