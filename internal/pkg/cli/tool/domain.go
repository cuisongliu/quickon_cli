// Copyright (c) 2021-2023 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
// Use of this source code is covered by the following dual licenses:
// (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
// (2) Affero General Public License 3.0 (AGPL 3.0)
// license that can be found in the LICENSE file.

package tool

import (
	"context"
	"fmt"
	"os"

	"github.com/easysoft/qcadmin/common"
	"github.com/easysoft/qcadmin/internal/app/config"
	"github.com/easysoft/qcadmin/internal/pkg/k8s"
	qcexec "github.com/easysoft/qcadmin/internal/pkg/util/exec"
	"github.com/easysoft/qcadmin/internal/pkg/util/factory"
	"github.com/easysoft/qcadmin/internal/pkg/util/helm"
	"github.com/easysoft/qcadmin/internal/pkg/util/kutil"
	"github.com/easysoft/qcadmin/pkg/qucheng/domain"
	suffixdomain "github.com/easysoft/qcadmin/pkg/qucheng/domain"
	"github.com/ergoapi/util/exmap"
	"github.com/ergoapi/util/exnet"
	"github.com/imroc/req/v3"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/strvals"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EmbedDomainCommand(f factory.Factory) *cobra.Command {
	domain := &cobra.Command{
		Use:    "domain",
		Short:  "domain manager",
		Hidden: true,
	}
	domain.AddCommand(domainClean(f))
	domain.AddCommand(domainAdd(f))
	return domain
}

func domainClean(f factory.Factory) *cobra.Command {
	dns := &cobra.Command{
		Use:    "clean",
		Short:  "clean domain",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, _ := config.LoadConfig()
			if cfg != nil {
				if !kutil.IsLegalDomain(cfg.Domain) {
					return
				}
			}
			secretKey := cfg.Cluster.ID
			if len(secretKey) == 0 {
				kclient, _ := k8s.NewSimpleClient()
				ns, err := kclient.GetNamespace(context.TODO(), common.DefaultKubeSystem, metav1.GetOptions{})
				if err != nil {
					return
				}
				secretKey = string(ns.ObjectMeta.GetUID())
				cfg.Cluster.ID = secretKey
				cfg.SaveConfig()
			}
			// TODO 获取subdomain, maindomain
			subDomain, mainDomain := kutil.SplitDomain(cfg.Domain)
			reqbody := domain.ReqBody{
				SecretKey:  secretKey,
				SubDomain:  subDomain,
				MainDomain: mainDomain,
			}
			client := req.C().SetLogger(nil).SetUserAgent(common.GetUG())
			if _, err := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(&reqbody).
				Delete(common.GetAPI("/api/qdnsv2/oss/record")); err != nil {
				f.GetLog().Error("clean dns failed, reason: %v", err)
			}
		},
	}
	return dns
}

func domainAdd(f factory.Factory) *cobra.Command {
	log := f.GetLog()
	var customdomain string
	dns := &cobra.Command{
		Use:    "init",
		Short:  "init domain",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			// load config
			domain := ""
			cfg, _ := config.LoadConfig()
			if cfg != nil {
				domain = cfg.Domain
			}
			if len(domain) > 0 {
				return
			}
			if len(customdomain) == 0 {
				kclient, _ := k8s.NewSimpleClient()
				ns, err := kclient.GetNamespace(context.TODO(), common.DefaultKubeSystem, metav1.GetOptions{})
				if err != nil {
					log.Errorf("conn k8s err: %v", err)
					return
				}
				secretKey := string(ns.ObjectMeta.GetUID())
				ip := exnet.LocalIPs()[0]
				// TODO
				domain, _, err = suffixdomain.GenerateDomain(ip, secretKey, suffixdomain.GenCustomDomain(suffixdomain.SearchCustomDomain(ip, secretKey, "")))
				if len(domain) == 0 {
					log.Warnf("gen domain failed: %v, use default domain: demo.corp.cc", err)
					domain = "demo.corp.cc"
				}
				cfg.Domain = domain
				cfg.Cluster.ID = secretKey
			} else {
				cfg.Domain = customdomain
			}
			// save config
			cfg.SaveConfig()
			// upgrade qucheng
			helmClient, _ := helm.NewClient(&helm.Config{Namespace: common.GetDefaultSystemNamespace(true)})
			if err := helmClient.UpdateRepo(); err != nil {
				log.Warnf("update repo failed, reason: %v", err)
			}
			if err := qcexec.Command(os.Args[0], "experimental", "kubectl", "apply", "-f", fmt.Sprintf("%s/hack/haogstls/haogs.yaml", common.GetDefaultDataDir()), "-n", common.GetDefaultSystemNamespace(true), "--kubeconfig", common.GetKubeConfig()).Run(); err != nil {
				log.Warnf("load tls cert for %s failed, reason: %v", common.GetDefaultSystemNamespace(true), err)
			} else {
				log.Donef("load tls cert for %s success", common.GetDefaultSystemNamespace(true))
			}
			if err := qcexec.Command(os.Args[0], "experimental", "kubectl", "apply", "-f", fmt.Sprintf("%s/hack/haogstls/haogs.yaml", common.GetDefaultDataDir()), "-n", "default", "--kubeconfig", common.GetKubeConfig()).Run(); err != nil {
				log.Warnf("load tls cert for default failed, reason: %v", err)
			} else {
				log.Done("load tls cert for default success")
			}
			defaultValue, _ := helmClient.GetValues(common.DefaultQuchengName)
			var values []string
			host := cfg.Domain
			if kutil.IsLegalDomain(host) {
				values = append(values, "ingress.tls.enabled=true")
				values = append(values, "ingress.tls.secretName=tls-haogs-cn")
			} else {
				host = fmt.Sprintf("console.%s", host)
			}
			values = append(values, fmt.Sprintf("ingress.host=%s", host), fmt.Sprintf("env.APP_DOMAIN=%s", cfg.Domain))
			base := map[string]interface{}{}
			for _, value := range values {
				strvals.ParseInto(value, base)
			}
			defaultValue = exmap.MergeMaps(defaultValue, base)
			if _, err := helmClient.Upgrade(common.DefaultQuchengName, common.DefaultHelmRepoName, common.DefaultQuchengName, "", defaultValue); err != nil {
				log.Warnf("upgrade %s failed, reason: %v", common.DefaultQuchengName, err)
			} else {
				log.Donef("upgrade %s success", common.DefaultQuchengName)
			}
		},
	}
	dns.Flags().StringVar(&customdomain, "domain", "", "app custom domain")
	return dns
}
