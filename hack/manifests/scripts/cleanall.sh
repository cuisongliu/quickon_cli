#!/bin/sh
# Copyright (c) 2021-2023 北京渠成软件有限公司(Beijing Qucheng Software Co., Ltd. www.qucheng.com) All rights reserved.
# Use of this source code is covered by the following dual licenses:
# (1) Z PUBLIC LICENSE 1.2 (ZPL 1.2)
# (2) Affero General Public License 3.0 (AGPL 3.0)
# license that can be found in the LICENSE file.


[ $(id -u) -eq 0 ] || exec sudo $0 $@

qcmd=${1:-"/usr/local/bin/qcadmin"}

if [ -f "${qcmd}" ]; then
  echo "${qcmd} clean doamin"
  ${qcmd} experimental tools domain clean
  echo "${qcmd} clean helm"
  ${qcmd} experimental helm uninstall --name cne-api -n cne-system
  ${qcmd} experimental helm uninstall --name qucheng -n cne-system
  ${qcmd} experimental helm repo-del
fi

for bin in /var/lib/rancher/k3s/data/**/bin/; do
    [ -d $bin ] && export PATH=$PATH:$bin:$bin/aux
done

for service in /etc/systemd/system/k3s*.service; do
    [ -s $service ] && systemctl stop $(basename $service)
done

for service in /etc/init.d/k3s*; do
    [ -x $service ] && $service stop
done

command_exists() {
	command -v "$@" > /dev/null 2>&1
}

pschildren() {
    ps -e -o ppid= -o pid= | \
    sed -e 's/^\s*//g; s/\s\s*/\t/g;' | \
    grep -w "^$1" | \
    cut -f2
}

pstree() {
    for pid in $@; do
        echo $pid
        for child in $(pschildren $pid); do
            pstree $child
        done
    done
}

killtree() {
    kill -9 $(
        { set +x; } 2>/dev/null;
        pstree $@;
        set -x;
    ) 2>/dev/null
}

getshims() {
    ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -w 'k3s/data/[^/]*/bin/containerd-shim' | cut -f1
}

killtree $({ set +x; } 2>/dev/null; getshims; set -x)

do_unmount_and_remove() {
    # set +x
    while read -r _ path _; do
        case "$path" in $1*) echo "$path" ;; esac
    done < /proc/self/mounts | sort -r | xargs -r -t -n 1 sh -c 'umount "$0" && rm -rf "$0"'
    # set -x
}

do_unmount_and_remove '/run/k3s'
do_unmount_and_remove '/var/lib/rancher/k3s'
do_unmount_and_remove '/var/lib/kubelet/pods'
do_unmount_and_remove '/var/lib/kubelet/plugins'
do_unmount_and_remove '/run/netns/cni-'

# Remove CNI namespaces
ip netns show 2>/dev/null | grep cni- | xargs -r -t -n 1 ip netns delete

# Delete network interface(s) that match 'master cni0'
ip link show 2>/dev/null | grep 'master cni0' | while read ignore iface ignore; do
    iface=${iface%%@*}
    [ -z "$iface" ] || ip link delete $iface
done
ip link delete cni0
ip link delete flannel.1
ip link delete flannel-v6.1
ip link delete kube-ipvs0
ip link delete flannel-wg
ip link delete flannel-wg-v6
rm -rf /var/lib/cni/
iptables-save | grep -v KUBE- | grep -v CNI- | grep -v flannel | iptables-restore
ip6tables-save | grep -v KUBE- | grep -v CNI- | grep -v flannel | ip6tables-restore

if command -v systemctl; then
		if [ -f "/etc/systemd/system/k3s.service" ]; then
				systemctl disable k3s.service
    		systemctl reset-failed k3s
				rm -f /etc/systemd/system/k3s.service
		fi
		[ -f "/etc/systemd/system/k3s.service.env" ] && rm -f /etc/systemd/system/k3s.service.env
    systemctl daemon-reload
fi

if command -v rc-update; then
    rc-update delete k3s-server default
fi
# remove_uninstall() {
#     rm -f /usr/local/bin/k3s-uninstall.sh
# }
# trap remove_uninstall EXIT

if (ls /etc/systemd/system/k3s*.service || ls /etc/init.d/k3s*) >/dev/null 2>&1; then
    set +x; echo 'Additional k3s services installed, skipping uninstall of k3s'; set -x
    exit
fi

for cmd in kubectl crictl ctr; do
    if [ -L /usr/local/bin/$cmd ]; then
        rm -f /usr/local/bin/$cmd
    fi
done

datadir=$(cat /root/.qc/config/cluster.yaml | grep datadir | awk '{print $2}')

[ ! -z "$datadir" ] && rm -rf $datadir

[ -d "/etc/rancher/k3s" ] && rm -rf /etc/rancher/k3s
[ -d "/run/k3s" ] && rm -rf /run/k3s
[ -d "/run/flannel" ] && rm -rf /run/flannel
[ -d "/var/lib/rancher/k3s" ] && rm -rf /var/lib/rancher/k3s
[ -d "/var/lib/kubelet" ] && rm -rf /var/lib/kubelet
[ -d "/opt/quickon" ] && rm -rf /opt/quickon
[ -d "/etc/quickon" ] && rm -rf /etc/quickon

# 暂时禁用selinux
# if type yum >/dev/null 2>&1; then
#     yum remove -y k3s-selinux
#     rm -f /etc/yum.repos.d/rancher-k3s-common*.repo
# fi

if [ -f "/usr/local/bin/k3s" ]; then
	rm -f /usr/local/bin/k3s
fi

# if [ -f "/usr/local/bin/helm" ]; then
# 	helm repo list | grep install && helm repo remove install || true
# 	rm -rf /usr/local/bin/helm
# fi

# clean kube config
if [ -f "/root/.kube/config" ]; then
	mv /root/.kube/config /root/.kube/config.bak
fi

if [ -d "/root/.qc/data" ]; then
	rm -rf /root/.qc/data
fi

if [ -d "/root/.qc/config" ]; then
	rm -rf /root/.qc/config
fi

if [ -d "/root/.qc/cache" ]; then
	rm -rf /root/.qc/cache
fi

if command_exists docker && [ -e /var/run/docker.sock ]; then
		(
			rm_ctns=$(docker ps -a -q --filter 'name=k8s')
			if [ -z "$rm_ctns" ];then
    		echo "no containers need to delete"
			else
        docker rm -f $rm_ctns
			fi
		) || true
fi

exit 0
