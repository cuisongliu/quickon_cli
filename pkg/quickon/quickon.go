// Copyright (c) 2021-2023 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
// Use of this source code is covered by the following dual licenses:
// (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
// (2) Affero General Public License 3.0 (AGPL 3.0)
// license that can be found in the LICENSE file.

package quickon

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/easysoft/qcadmin/internal/pkg/types"

	"github.com/cockroachdb/errors"
	"github.com/easysoft/qcadmin/common"
	"github.com/easysoft/qcadmin/internal/app/config"
	"github.com/easysoft/qcadmin/internal/pkg/k8s"
	qcexec "github.com/easysoft/qcadmin/internal/pkg/util/exec"
	"github.com/easysoft/qcadmin/internal/pkg/util/factory"
	"github.com/easysoft/qcadmin/internal/pkg/util/kutil"
	"github.com/easysoft/qcadmin/internal/pkg/util/log"
	"github.com/easysoft/qcadmin/internal/pkg/util/retry"
	suffixdomain "github.com/easysoft/qcadmin/pkg/qucheng/domain"
	"github.com/ergoapi/util/color"
	"github.com/ergoapi/util/exnet"
	"github.com/ergoapi/util/expass"
	"github.com/ergoapi/util/file"
	"github.com/ergoapi/util/ztime"
	"github.com/imroc/req/v3"
	"golang.org/x/sync/errgroup"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Meta struct {
	Domain          string
	IP              string
	Version         string
	ConsolePassword string
	DevopsMode      bool
	OffLine         bool
	QuickonOSS      bool
	QuickonType     common.QuickonType
	kubeClient      *k8s.Client
	Log             log.Logger
}

func New(f factory.Factory) *Meta {
	return &Meta{
		Log: f.GetLog(),
		// Version:         common.DefaultQuickonOSSVersion,
		ConsolePassword: expass.PwGenAlphaNum(32),
		QuickonType:     common.QuickonOSSType,
	}
}

func (m *Meta) GetCustomFlags() []types.Flag {
	return []types.Flag{
		{
			Name:  "domain",
			Usage: "quickon domain",
			P:     &m.Domain,
			V:     m.Domain,
		},
		{
			Name:   "ip",
			Usage:  "quickon ip",
			P:      &m.IP,
			V:      m.IP,
			Hidden: true,
		},
		{
			Name:  "oss",
			Usage: "type, oss or ee, default: oss",
			P:     &m.QuickonOSS,
			V:     m.QuickonType == common.QuickonOSSType,
		},
		{
			Name:  "offline",
			Usage: "offline install mode, default: false",
			P:     &m.OffLine,
			V:     false,
		},
	}
}

func (m *Meta) GetKubeClient() error {
	kubeClient, err := k8s.NewSimpleClient(common.GetKubeConfig())
	if err != nil {
		return errors.Errorf("load k8s client failed, reason: %v", err)
	}
	m.kubeClient = kubeClient
	return nil
}

func (m *Meta) checkIngress() {
	m.Log.StartWait("check default ingress class")
	defaultClass, _ := m.kubeClient.ListDefaultIngressClass(context.Background(), metav1.ListOptions{})
	m.Log.StopWait()
	if defaultClass == nil {
		m.Log.Infof("not found default ingress class, will install nginx ingress")
		m.Log.Debug("start install default ingress: nginx-ingress-controller")
		if err := qcexec.CommandRun(os.Args[0], "quickon", "plugins", "enable", "ingress"); err != nil {
			m.Log.Errorf("install ingress failed, reason: %v", err)
		} else {
			m.Log.Done("install ingress: cne-ingress success")
		}
	} else {
		m.Log.Infof("found exist default ingress class: %s", defaultClass.Name)
	}
	m.Log.Done("check default ingress done")
}

func (m *Meta) checkStorage() {
	m.Log.StartWait("check default storage class")
	defaultClass, _ := m.kubeClient.GetDefaultSC(context.Background())
	m.Log.StopWait()
	if defaultClass == nil {
		// TODO default storage
		m.Log.Infof("not found default storage class, will install default storage")
		m.Log.Debug("start install default storage: longhorn")
		if err := qcexec.CommandRun(os.Args[0], "cluster", "storage", "longhorn"); err != nil {
			m.Log.Errorf("install storage failed, reason: %v", err)
		} else {
			m.Log.Done("install storage: longhorn success")
		}
		if err := qcexec.CommandRun(os.Args[0], "cluster", "storage", "set-default"); err != nil {
			m.Log.Errorf("set default storageclass failed, reason: %v", err)
		}
	} else {
		m.Log.Infof("found exist default storage class: %s", defaultClass.Name)
	}
	m.Log.Done("check default storage done")
}

func (m *Meta) Check() error {
	if err := m.addHelmRepo(); err != nil {
		return err
	}
	if err := m.initNS(); err != nil {
		return err
	}
	m.checkIngress()
	m.checkStorage()
	return nil
}

func (m *Meta) initNS() error {
	m.Log.Debugf("init quickon default namespace.")
	for _, ns := range common.GetDefaultQuickONNamespace() {
		_, err := m.kubeClient.GetNamespace(context.TODO(), ns, metav1.GetOptions{})
		if err != nil {
			if !kubeerr.IsNotFound(err) {
				return err
			}
			if _, err := m.kubeClient.CreateNamespace(context.TODO(), ns, metav1.CreateOptions{}); err != nil && kubeerr.IsAlreadyExists(err) {
				return err
			}
		}
	}
	m.Log.Donef("init quickon default namespace success.")
	return nil
}

func (m *Meta) addHelmRepo() error {
	output, err := qcexec.Command(os.Args[0], "experimental", "helm", "repo-add", "--name", common.DefaultHelmRepoName, "--url", common.GetChartRepo(m.Version)).CombinedOutput()
	if err != nil {
		errmsg := string(output)
		if !strings.Contains(errmsg, "exists") {
			m.Log.Errorf("init quickon helm repo failed, reason: %s", string(output))
			return err
		}
		m.Log.Debugf("quickon helm repo already exists")
	} else {
		m.Log.Done("add quickon helm repo success")
	}
	output, err = qcexec.Command(os.Args[0], "experimental", "helm", "repo-update").CombinedOutput()
	if err != nil {
		m.Log.Errorf("update quickon helm repo failed, reason: %s", string(output))
		return err
	}
	m.Log.Done("update quickon helm repo success")
	return nil
}

func (m *Meta) Init() error {
	cfg, _ := config.LoadConfig()
	m.Log.Info("executing init logic...")
	ctx := context.Background()
	m.Log.Debug("waiting for storage to be ready...")
	waitsc := time.Now()
	// wait.BackoffUntil TODO
	for {
		sc, _ := m.kubeClient.GetDefaultSC(ctx)
		if sc != nil {
			m.Log.Donef("default storage %s is ready", sc.Name)
			break
		}
		time.Sleep(time.Second * 5)
		trywaitsc := time.Now()
		if trywaitsc.Sub(waitsc) > time.Minute*3 {
			m.Log.Warnf("wait storage ready, timeout: %v", trywaitsc.Sub(waitsc).Seconds())
			break
		}
	}

	_, err := m.kubeClient.CreateNamespace(ctx, common.GetDefaultSystemNamespace(true), metav1.CreateOptions{})
	if err != nil {
		if !kubeerr.IsAlreadyExists(err) {
			return err
		}
	}
	chartVersion := common.GetVersion(m.Version, m.QuickonType)
	if m.DevopsMode {
		// TODO: 获取zentao devops chart version
		chartVersion = m.Version
		m.Log.Debugf("start init zentao devops, version: %s", chartVersion)
		cfg.Quickon.Type = m.QuickonType
		cfg.Quickon.DevOps = true
	} else {
		m.Log.Debugf("start init quickon %v, version: %s", m.QuickonType, chartVersion)
		cfg.Quickon.Type = m.QuickonType
	}
	if m.Domain == "" {
		err := retry.Retry(time.Second*1, 3, func() (bool, error) {
			domain, _, err := m.genSuffixHTTPHost(m.IP)
			if err != nil {
				return false, err
			}
			m.Domain = domain
			m.Log.Infof("generate suffix domain: %s, ip: %v", color.SGreen(m.Domain), color.SGreen(m.IP))
			return true, nil
		})
		if err != nil {
			m.Domain = "demo.corp.cc"
			m.Log.Warnf("gen suffix domain failed, reason: %v, use default domain: %s", err, m.Domain)
		}
		if kutil.IsLegalDomain(m.Domain) {
			m.Log.Infof("load %s tls cert", m.Domain)
			defaultTLS := fmt.Sprintf("%s/tls-haogs-cn.yaml", common.GetDefaultCacheDir())
			m.Log.StartWait(fmt.Sprintf("start issuing domain %s certificate, may take 3-5min", m.Domain))
			waittls := time.Now()
			for {
				if file.CheckFileExists(defaultTLS) {
					m.Log.StopWait()
					m.Log.Done("download tls cert success")
					if err := qcexec.Command(os.Args[0], "experimental", "kubectl", "apply", "-f", defaultTLS, "-n", common.GetDefaultSystemNamespace(true), "--kubeconfig", common.GetKubeConfig()).Run(); err != nil {
						m.Log.Warnf("load default tls cert failed, reason: %v", err)
					} else {
						m.Log.Done("load default tls cert success")
					}
					qcexec.Command(os.Args[0], "experimental", "kubectl", "apply", "-f", defaultTLS, "-n", "default", "--kubeconfig", common.GetKubeConfig()).Run()
					break
				}
				_, mainDomain := kutil.SplitDomain(m.Domain)
				domainTLS := fmt.Sprintf("https://pkg.qucheng.com/ssl/%s/%s/tls.yaml", mainDomain, m.Domain)
				qcexec.Command(os.Args[0], "experimental", "tools", "wget", "-t", domainTLS, "-d", defaultTLS).Run()
				m.Log.Debug("wait for tls cert ready...")
				time.Sleep(time.Second * 5)
				trywaitsc := time.Now()
				if trywaitsc.Sub(waittls) > time.Minute*3 {
					// TODO  timeout
					m.Log.Debugf("wait tls cert ready, timeout: %v", trywaitsc.Sub(waittls).Seconds())
					break
				}
			}
		} else {
			m.Log.Infof("use custom domain %s, you should add dns record to your domain: *.%s -> %s", m.Domain, color.SGreen(m.Domain), color.SGreen(m.IP))
		}
	} else {
		m.Log.Infof("use custom domain %s, you should add dns record to your domain: *.%s -> %s", m.Domain, color.SGreen(m.Domain), color.SGreen(m.IP))
	}
	token := expass.PwGenAlphaNum(32)

	cfg.Domain = m.Domain
	cfg.APIToken = token
	cfg.S3.Username = expass.PwGenAlphaNum(8)
	cfg.S3.Password = expass.PwGenAlphaNum(16)
	cfg.SaveConfig()
	m.Log.Info("start deploy custom tools")
	toolargs := []string{"experimental", "helm", "upgrade", "--name", "selfcert", "--repo", common.DefaultHelmRepoName, "--chart", "selfcert", "--namespace", common.GetDefaultSystemNamespace(true)}
	if helmstd, err := qcexec.Command(os.Args[0], toolargs...).CombinedOutput(); err != nil {
		m.Log.Warnf("deploy custom tools err: %v, std: %s", err, string(helmstd))
	} else {
		m.Log.Done("deployed custom tools success")
	}
	m.Log.Info("start deploy operator")
	operatorargs := []string{"experimental", "helm", "upgrade", "--name", common.DefaultCneOperatorName, "--repo", common.DefaultHelmRepoName, "--chart", common.DefaultCneOperatorName, "--namespace", common.GetDefaultSystemNamespace(true),
		"--set", "minio.ingress.enabled=true",
		"--set", "minio.ingress.host=s3." + m.Domain,
		"--set", "minio.auth.username=" + cfg.S3.Username,
		"--set", "minio.auth.password=" + cfg.S3.Password,
	}
	if helmstd, err := qcexec.Command(os.Args[0], operatorargs...).CombinedOutput(); err != nil {
		m.Log.Warnf("deploy operator err: %v, std: %s", err, string(helmstd))
	} else {
		m.Log.Done("deployed operator success")
	}
	helmchan := common.GetChannel(m.Version)
	helmargs := []string{"experimental", "helm", "upgrade", "--name", common.DefaultQuchengName, "--repo", common.DefaultHelmRepoName, "--chart", common.GetQuickONName(m.QuickonType), "--namespace", common.GetDefaultSystemNamespace(true), "--set", "env.APP_DOMAIN=" + m.Domain, "--set", "env.CNE_API_TOKEN=" + token, "--set", "cloud.defaultChannel=" + helmchan}
	if helmchan != "stable" {
		helmargs = append(helmargs, "--set", "env.PHP_DEBUG=2")
		helmargs = append(helmargs, "--set", "cloud.switchChannel=true")
		helmargs = append(helmargs, "--set", "cloud.selectVersion=true")
	}
	hostdomain := m.Domain
	if kutil.IsLegalDomain(hostdomain) {
		helmargs = append(helmargs, "--set", "ingress.tls.enabled=true")
		helmargs = append(helmargs, "--set", "ingress.tls.secretName=tls-haogs-cn")
	} else {
		if m.DevopsMode {
			hostdomain = fmt.Sprintf("zentao.%s", hostdomain)
		} else {
			hostdomain = fmt.Sprintf("console.%s", hostdomain)
		}
	}

	if m.OffLine {
		helmargs = append(helmargs, "--set", fmt.Sprintf("cloud.host=http://market-cne-market-api.quickon-system.svc:8088"))
		helmargs = append(helmargs, "--set", fmt.Sprintf("env.CNE_MARKET_API_SCHEMA=http"))
		helmargs = append(helmargs, "--set", fmt.Sprintf("env.CNE_MARKET_API_HOST=market-cne-market-api.quickon-system.svc"))
		helmargs = append(helmargs, "--set", fmt.Sprintf("env.CNE_MARKET_API_PORT=8088"))
	}

	helmargs = append(helmargs, "--set", fmt.Sprintf("ingress.host=%s", hostdomain))

	if len(chartVersion) > 0 {
		helmargs = append(helmargs, "--version", chartVersion)
	}
	output, err := qcexec.Command(os.Args[0], helmargs...).CombinedOutput()
	if err != nil {
		m.Log.Errorf("upgrade install web failed: %s", string(output))
		return err
	}
	m.Log.Done("install success")
	if m.OffLine {
		// patch quickon
		cmfileName := fmt.Sprintf("%s-%s-files", common.DefaultQuchengName, common.GetQuickONName(m.QuickonType))
		m.Log.Debugf("fetch quickon files from %s", cmfileName)
		for i := 0; i < 20; i++ {
			time.Sleep(5 * time.Second)
			foundRepofiles, _ := m.kubeClient.GetConfigMap(ctx, common.GetDefaultSystemNamespace(true), cmfileName, metav1.GetOptions{})
			if foundRepofiles != nil {
				foundRepofiles.Data["repositories.yaml"] = fmt.Sprintf(`apiVersion: ""
generated: "0001-01-01T00:00:00Z"
repositories:
- caFile: ""
  certFile: ""
  insecure_skip_tls_verify: true
  keyFile: ""
  name: qucheng-stable
  pass_credentials_all: false
  password: ""
  url: http://%s:32377
  username: ""
`, exnet.LocalIPs()[0])
				_, err := m.kubeClient.UpdateConfigMap(ctx, foundRepofiles, metav1.UpdateOptions{})
				if err != nil {
					m.Log.Warnf("patch offline repo file, check: kubectl get cm/%s  -n %s", cmfileName, common.GetDefaultSystemNamespace(true))
				}
				// 重建pod
				pods, _ := m.kubeClient.ListPods(ctx, common.GetDefaultSystemNamespace(true), metav1.ListOptions{})
				podName := fmt.Sprintf("%s-%s", common.DefaultQuchengName, common.GetQuickONName(m.QuickonType))
				for _, pod := range pods.Items {
					if strings.HasPrefix(pod.Name, podName) {
						if err := m.kubeClient.DeletePod(ctx, pod.Name, common.GetDefaultSystemNamespace(true), metav1.DeleteOptions{}); err != nil {
							m.Log.Warnf("recreate quickon pods")
						}
					}
				}
				break
			}
		}

		// install cne-market
		m.Log.Infof("start deploy cloudapp market")
		marketargs := []string{"experimental", "helm", "upgrade", "--name", "market", "--repo", common.DefaultHelmRepoName, "--chart", "cne-market-api", "--namespace", common.GetDefaultSystemNamespace(true)}
		output, err := qcexec.Command(os.Args[0], marketargs...).CombinedOutput()
		if err != nil {
			m.Log.Warnf("upgrade install cloudapp market failed: %s", string(output))
		}
	}
	m.QuickONReady()
	initFile := common.GetCustomConfig(common.InitFileName)
	if err := file.WriteFile(initFile, "init done", true); err != nil {
		m.Log.Warnf("write init done file failed, reason: %v.\n\t please run: touch %s", err, initFile)
	}
	m.Show()
	return nil
}

// QuickONReady 渠成Ready
func (m *Meta) QuickONReady() {
	clusterWaitGroup, ctx := errgroup.WithContext(context.Background())
	clusterWaitGroup.Go(func() error {
		return m.readyQuickON(ctx)
	})
	if err := clusterWaitGroup.Wait(); err != nil {
		m.Log.Error(err)
	}
}

func (m *Meta) readyQuickON(ctx context.Context) error {
	t1 := ztime.NowUnix()
	client := req.C().SetLogger(nil).SetUserAgent(common.GetUG()).SetTimeout(time.Second * 1)
	m.Log.StartWait("waiting for quickon ready")
	status := false
	for {
		t2 := ztime.NowUnix() - t1
		if t2 > 180 {
			m.Log.Warnf("waiting for quickon ready 3min timeout: check your network or storage. after install you can run: q status")
			break
		}
		_, err := client.R().Get(fmt.Sprintf("http://%s:32379", exnet.LocalIPs()[0]))
		if err == nil {
			status = true
			break
		}
		time.Sleep(time.Second * 10)
	}
	m.Log.StopWait()
	if status {
		m.Log.Donef("quickon ready, cost: %v", time.Since(time.Unix(t1, 0)))
	}
	return nil
}

func (m *Meta) getOrCreateUUIDAndAuth() (auth string, err error) {
	ns, err := m.kubeClient.GetNamespace(context.TODO(), common.DefaultKubeSystem, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(ns.GetUID()), nil
}

func (m *Meta) genSuffixHTTPHost(ip string) (domain, tls string, err error) {
	auth, err := m.getOrCreateUUIDAndAuth()
	if err != nil {
		return "", "", err
	}
	defaultDomain := suffixdomain.SearchCustomDomain(ip, auth, "")
	domain, tls, err = suffixdomain.GenerateDomain(ip, auth, suffixdomain.GenCustomDomain(defaultDomain))
	if err != nil {
		return "", "", err
	}
	return domain, tls, nil
}

func (m *Meta) Show() {
	if len(m.IP) <= 0 {
		m.IP = exnet.LocalIPs()[0]
	}
	resetPassArgs := []string{"quickon", "reset-password", "--password", m.ConsolePassword}
	qcexec.CommandRun(os.Args[0], resetPassArgs...)
	cfg, _ := config.LoadConfig()
	cfg.ConsolePassword = m.ConsolePassword
	cfg.SaveConfig()
	domain := cfg.Domain

	m.Log.Info("----------------------------\t")
	if len(domain) > 0 {
		if !kutil.IsLegalDomain(cfg.Domain) {
			domain = fmt.Sprintf("http://console.%s", cfg.Domain)
		} else {
			domain = fmt.Sprintf("https://%s", cfg.Domain)
		}
	} else {
		domain = fmt.Sprintf("http://%s:32379", m.IP)
	}
	m.Log.Donef("console: %s, username: %s, password: %s",
		color.SGreen(domain), color.SGreen(common.QuchengDefaultUser), color.SGreen(m.ConsolePassword))
	m.Log.Donef("docs: %s", common.QuchengDocs)
	m.Log.Done("support: 768721743(QQGroup)")
}

func (m *Meta) UnInstall() error {
	m.Log.Warnf("start clean quickon.")
	cfg, _ := config.LoadConfig()
	// 清理helm安装应用
	m.Log.Info("start uninstall cne custom tools")
	toolArgs := []string{"experimental", "helm", "uninstall", "--name", "selfcert", "--namespace", common.GetDefaultSystemNamespace(true)}
	if cleanStd, err := qcexec.Command(os.Args[0], toolArgs...).CombinedOutput(); err != nil {
		m.Log.Warnf("uninstall cne custom tools err: %v, std: %s", err, string(cleanStd))
	} else {
		m.Log.Done("uninstall cne custom tools success")
	}
	m.Log.Info("start uninstall cne operator")
	operatorArgs := []string{"experimental", "helm", "uninstall", "--name", common.DefaultCneOperatorName, "--namespace", common.GetDefaultSystemNamespace(true)}
	if cleanStd, err := qcexec.Command(os.Args[0], operatorArgs...).CombinedOutput(); err != nil {
		m.Log.Warnf("uninstall cne-operator err: %v, std: %s", err, string(cleanStd))
	} else {
		m.Log.Done("uninstall cne-operator success")
	}
	m.Log.Info("start uninstall cne quickon")
	quickonCleanArgs := []string{"experimental", "helm", "uninstall", "--name", common.DefaultQuchengName, "--namespace", common.GetDefaultSystemNamespace(true)}
	if cleanStd, err := qcexec.Command(os.Args[0], quickonCleanArgs...).CombinedOutput(); err != nil {
		m.Log.Warnf("uninstall quickon err: %v, std: %s", err, string(cleanStd))
	} else {
		m.Log.Done("uninstall quickon success")
	}
	m.Log.Info("start uninstall helm repo")
	repoCleanArgs := []string{"experimental", "helm", "repo-del"}
	_ = qcexec.Command(os.Args[0], repoCleanArgs...).Run()
	m.Log.Done("uninstall helm repo success")
	if strings.HasSuffix(cfg.Domain, "haogs.cn") || strings.HasSuffix(cfg.Domain, "corp.cc") {
		m.Log.Infof("clean domain %s", cfg.Domain)
		if err := qcexec.Command(os.Args[0], "exp", "tools", "domain", "clean", cfg.Domain).Run(); err != nil {
			m.Log.Warnf("clean domain %s failed, reason: %v", cfg.Domain, err)
		}
	}
	f := common.GetCustomConfig(common.InitFileName)
	if file.CheckFileExists(f) {
		os.Remove(f)
	}
	return nil
}
