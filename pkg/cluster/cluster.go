// Copyright (c) 2021-2023 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
// Use of this source code is covered by the following dual licenses:
// (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
// (2) Affero General Public License 3.0 (AGPL 3.0)
// license that can be found in the LICENSE file.

package cluster

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/cockroachdb/errors"

	"github.com/easysoft/qcadmin/common"
	"github.com/easysoft/qcadmin/internal/app/config"
	"github.com/easysoft/qcadmin/internal/pkg/cli/k3stpl"
	"github.com/easysoft/qcadmin/internal/pkg/k8s"
	"github.com/easysoft/qcadmin/internal/pkg/types"
	"github.com/easysoft/qcadmin/internal/pkg/util/log"
	"github.com/easysoft/qcadmin/internal/pkg/util/ssh"
	"github.com/ergoapi/util/exid"
	"github.com/ergoapi/util/expass"
	"github.com/ergoapi/util/exstr"
	"github.com/ergoapi/util/file"

	"github.com/easysoft/qcadmin/internal/pkg/util/factory"
)

var defaultBackoff = wait.Backoff{
	Duration: 6 * time.Second,
	Factor:   1,
	Steps:    10,
}

type Cluster struct {
	log         log.Logger
	MasterIPs   []string
	WorkerIPs   []string
	IPs         []string
	SSH         types.SSH
	CNI         string
	DataDir     string
	PodCIDR     string
	ServiceCIDR string
}

func NewCluster(f factory.Factory) *Cluster {
	return &Cluster{
		log: f.GetLog(),
	}
}

func (c *Cluster) preinit(mip, ip string, sshClient ssh.Interface) error {
	k3sbin := fmt.Sprintf("%s/hack/bin/k3s-%s-%s", common.GetDefaultDataDir(), runtime.GOOS, runtime.GOARCH)
	if err := sshClient.Copy(ip, k3sbin, "/usr/local/bin/k3s"); err != nil {
		return errors.Errorf("copy k3s bin (%s:%s -> %s:/usr/local/bin/k3s) failed, reason: %v", ip, mip, k3sbin, ip, err)
	}
	qbin, _ := os.Executable()
	if err := sshClient.Copy(ip, qbin, "/usr/local/bin/qcadmin"); err != nil {
		return errors.Errorf("copy k3s bin (%s:%s -> %s:/usr/local/bin/qcadmin) failed, reason: %v", ip, mip, qbin, ip, err)
	}
	if err := sshClient.CmdAsync(ip, "/usr/local/bin/qcadmin version"); err != nil {
		return errors.Errorf("load q version failed, reason: %v", err)
	}
	c.log.StartWait(ip + " start run init script")
	if err := sshClient.CmdAsync(ip, "/root/.qc/data/hack/manifests/scripts/init.sh"); err != nil {
		return errors.Errorf("%s run init script failed, reason: %v", ip, err)
	}
	c.log.StopWait()
	c.log.Donef("%s run init script success", ip)
	// add master0 ip
	hostsArgs := fmt.Sprintf("/usr/local/bin/qcadmin exp tools hosts add --domain kubeapi.k7s.local --ip %s", mip)
	if err := sshClient.CmdAsync(ip, hostsArgs); err != nil {
		c.log.Debugf("cmd: %s", hostsArgs)
		return errors.Errorf("%s add master0 (kubeapi.k7s.local --> %s) failed, reason: %v", ip, mip, err)
	}
	if err := sshClient.CmdAsync(ip, "/root/.qc/data/hack/manifests/scripts/node.sh"); err != nil {
		return errors.Errorf("%s run init script failed, reason: %v", ip, err)
	}
	return nil
}

func (c *Cluster) initMaster0(cfg *config.Config, sshClient ssh.Interface) error {
	c.log.Infof("master0 ip: %s", cfg.Cluster.InitNode)
	k3sargs := k3stpl.K3sArgs{
		Master0:     true,
		TypeMaster:  true,
		KubeAPI:     "kubeapi.k7s.local",
		KubeToken:   expass.PwGenAlphaNum(16),
		DataDir:     c.DataDir,
		PodCIDR:     c.PodCIDR,
		ServiceCIDR: c.ServiceCIDR,
		CNI:         c.CNI,
		// TODO EE
		DataStore:    "",
		LocalStorage: true,
	}
	master0tplSrc := fmt.Sprintf("%s/master0.%s", common.GetDefaultCacheDir(), cfg.Cluster.InitNode)
	master0tplDst := fmt.Sprintf("/%s/.k3s.service", c.SSH.User)
	file.Writefile(master0tplSrc, k3sargs.Manifests(""), true)
	if err := sshClient.Copy(cfg.Cluster.InitNode, master0tplSrc, master0tplDst); err != nil {
		return errors.Errorf("copy master0 %s tpl failed, reason: %v", cfg.Cluster.InitNode, err)
	}
	if err := c.preinit(cfg.Cluster.InitNode, cfg.Cluster.InitNode, sshClient); err != nil {
		return err
	}
	// waiting k3s ready
	if err := c.waitk3sReady(cfg.Cluster.InitNode, sshClient); err != nil {
		return err
	}
	cfg.Cluster.ID = exid.GenUUID()
	cfg.Cluster.PodCIDR = c.PodCIDR
	cfg.Cluster.ServiceCIDR = c.ServiceCIDR
	cfg.Cluster.CNI = c.CNI
	cfg.DB = k3sargs.DataStore
	cfg.DataDir = k3sargs.DataDir
	cfg.Cluster.Master = append(cfg.Cluster.Master, config.Node{
		Host: cfg.Cluster.InitNode,
		Init: true,
	})
	cfg.Cluster.Token = k3sargs.KubeToken
	return cfg.SaveConfig()
}

func (c *Cluster) waitk3sReady(host string, sshClient ssh.Interface) error {
	c.log.StartWait("check k8s ready.")
	try := 0
	err := wait.ExponentialBackoff(defaultBackoff, func() (bool, error) {
		try++
		c.log.Debugf("the %d/%d time tring to check k8s status", try, defaultBackoff.Steps)
		// TODO 地址错误问题
		err := sshClient.Copy(host, "/etc/rancher/k3s/k3s.yaml", common.GetDefaultNewKubeConfig())
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	c.log.StopWait()
	if err != nil {
		return fmt.Errorf("check k8s ready failed, reason: %w", err)
	}
	c.log.Done("check k8s ready.")
	return nil
}

func (c *Cluster) joinNode(ip string, master bool, cfg *config.Config, sshClient ssh.Interface) error {
	t := "worker"
	if master {
		t = "master"
	}
	c.log.Infof("node role: %s, ip: %s", t, ip)
	k3sargs := k3stpl.K3sArgs{
		Master0:      false,
		TypeMaster:   master,
		KubeAPI:      "kubeapi.k7s.local",
		KubeToken:    cfg.Cluster.Token,
		DataDir:      cfg.DataDir,
		PodCIDR:      "",
		ServiceCIDR:  "",
		DataStore:    cfg.DB,
		LocalStorage: true,
	}
	tplSrc := fmt.Sprintf("%s/%s.%s", common.GetDefaultCacheDir(), t, ip)
	tplDst := fmt.Sprintf("/%s/.k3s.service", c.SSH.User)
	file.Writefile(tplSrc, k3sargs.Manifests(""), true)
	if err := sshClient.Copy(ip, tplSrc, tplDst); err != nil {
		return errors.Errorf("%s copy tpl (%s:%s->%s:%s) failed, reason: %v", t, cfg.Cluster.InitNode, tplSrc, ip, tplDst, err)
	}
	if err := c.preinit(cfg.Cluster.InitNode, ip, sshClient); err != nil {
		return err
	}
	if master {
		cfg.Cluster.Master = append(cfg.Cluster.Master, config.Node{
			Host: ip,
		})
	} else {
		cfg.Cluster.Worker = append(cfg.Cluster.Worker, config.Node{
			Host: ip,
		})
	}
	return cfg.SaveConfig()
}

func (c *Cluster) InitNode() error {
	c.log.Info("init node")
	c.MasterIPs = exstr.DuplicateStrElement(c.MasterIPs)
	c.WorkerIPs = exstr.DuplicateStrElement(c.WorkerIPs)
	otherMaster := c.MasterIPs[1:]
	sshClient := ssh.NewSSHClient(&c.SSH, true)
	cfg := config.LoadTruncateConfig()
	cfg.Cluster.InitNode = c.MasterIPs[0]
	if err := c.initMaster0(cfg, sshClient); err != nil {
		return err
	}
	for _, host := range otherMaster {
		c.log.Debugf("ping master %s", host)
		if err := sshClient.Ping(host); err != nil {
			c.log.Warnf("skip join master: %s, reason: %v", host, err)
			continue
		}
		if err := c.joinNode(host, true, cfg, sshClient); err != nil {
			c.log.Warnf("skip join master: %s, reason: %v", host, err)
		}
	}
	for _, host := range c.WorkerIPs {
		c.log.Debugf("ping worker %s", host)
		if err := sshClient.Ping(host); err != nil {
			c.log.Warnf("skip join worker: %s, reason: %v", host, err)
			continue
		}
		if err := c.joinNode(host, false, cfg, sshClient); err != nil {
			c.log.Warnf("skip join worker: %s, reason: %v", host, err)
		}
	}
	return nil
}

func (c *Cluster) JoinNode() error {
	c.log.Info("join node")
	c.MasterIPs = exstr.DuplicateStrElement(c.MasterIPs)
	c.WorkerIPs = exstr.DuplicateStrElement(c.WorkerIPs)
	sshClient := ssh.NewSSHClient(&c.SSH, true)
	cfg, _ := config.LoadConfig()
	for _, host := range c.MasterIPs {
		c.log.Debugf("ping master %s", host)
		if err := sshClient.Ping(host); err != nil {
			c.log.Warnf("skip join master: %s, reason: %v", host, err)
			continue
		}
		if err := c.joinNode(host, true, cfg, sshClient); err != nil {
			c.log.Warnf("skip join master: %s, reason: %v", host, err)
		}
	}
	for _, host := range c.WorkerIPs {
		c.log.Debugf("ping worker %s", host)
		if err := sshClient.Ping(host); err != nil {
			c.log.Warnf("skip join worker: %s, reason: %v", host, err)
			continue
		}
		if err := c.joinNode(host, false, cfg, sshClient); err != nil {
			c.log.Warnf("skip join worker: %s, reason: %v", host, err)
		}
	}
	return nil
}

func (c *Cluster) cleanNode(ip string, sshClient ssh.Interface, wg *sync.WaitGroup) {
	defer wg.Done()
	c.log.StartWait(fmt.Sprintf("start clean node: %s", ip))
	err := sshClient.CmdAsync(ip, "/root/.qc/data/hack/manifests/scripts/cleankube.sh")
	c.log.StopWait()
	if err != nil {
		c.log.Warnf("clean node %s failed, reason: %v", ip, err)
		return
	}
	c.log.Donef("clean node %s success", ip)
}

func (c *Cluster) deleteNode(ip string, sshClient ssh.Interface, kubeClient *k8s.Client, wg *sync.WaitGroup) error {
	c.log.Infof("start clean node %s", ip)
	// 从集群中移除节点
	c.log.Infof("delete node %s from cluster", ip)
	if err := kubeClient.DownNode(context.TODO(), ip); err != nil {
		c.log.Warnf("delete node %s from cluster failed, reason: %v", ip, err)
	}
	// 清理节点
	c.cleanNode(ip, sshClient, wg)
	return nil
}

func (c *Cluster) DeleteNode() error {
	cfg, _ := config.LoadConfig()
	var wg sync.WaitGroup
	sshClient := ssh.NewSSHClient(&cfg.Global.SSH, true)
	kubeClient, err := k8s.NewSimpleClient(common.GetDefaultNewKubeConfig())
	if err != nil {
		return errors.Errorf("load k8s client failed, reason: %v", err)
	}
	for _, ip := range c.IPs {
		if ip == cfg.Cluster.InitNode {
			c.log.Warnf("init node %s not allow delete, can use clean subcmd", ip)
			continue
		}
		wg.Add(1)
		c.deleteNode(ip, sshClient, kubeClient, &wg)
	}
	wg.Wait()
	return cfg.SaveConfig()
}

// Clean 清理集群
func (c *Cluster) Clean() error {
	c.log.Info("start clean cluster")
	cfg, _ := config.LoadConfig()
	sshClient := ssh.NewSSHClient(&cfg.Global.SSH, true)
	var wg sync.WaitGroup
	for _, ip := range cfg.GetIPs() {
		wg.Add(1)
		go c.cleanNode(ip, sshClient, &wg)
	}
	wg.Wait()
	c.log.Done("clean cluster success")
	return nil
}
