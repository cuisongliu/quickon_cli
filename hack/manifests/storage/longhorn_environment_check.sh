#!/bin/bash

NVME_CLI_VERSION="1.12"

######################################################
# Log
######################################################
export RED='\x1b[0;31m'
export GREEN='\x1b[38;5;22m'
export CYAN='\x1b[36m'
export YELLOW='\x1b[33m'
export NO_COLOR='\x1b[0m'

if [ -z "${LOG_TITLE}" ]; then
  LOG_TITLE=''
fi
if [ -z "${LOG_LEVEL}" ]; then
  LOG_LEVEL="INFO"
fi

debug() {
  if [[ "${LOG_LEVEL}" == "DEBUG" ]]; then
    local log_title
    if [ -n "${LOG_TITLE}" ]; then
     log_title="(${LOG_TITLE})"
    else
     log_title=''
    fi
    echo -e "${GREEN}[DEBUG]${log_title} ${NO_COLOR}$1"
  fi
}

info() {
  if [[ "${LOG_LEVEL}" == "DEBUG" ]] ||\
     [[ "${LOG_LEVEL}" == "INFO" ]]; then
    local log_title
    if [ -n "${LOG_TITLE}" ]; then
     log_title="(${LOG_TITLE})"
    else
     log_title=''
    fi
    echo -e "${CYAN}[INFO]  ${log_title} ${NO_COLOR}$1"
  fi
}

warn() {
  if [[ "${LOG_LEVEL}" == "DEBUG" ]] ||\
     [[ "${LOG_LEVEL}" == "INFO" ]] ||\
     [[ "${LOG_LEVEL}" == "WARN" ]]; then
    local log_title
    if [ -n "${LOG_TITLE}" ]; then
     log_title="(${LOG_TITLE})"
    else
     log_title=''
    fi
    echo -e "${YELLOW}[WARN]  ${log_title} ${NO_COLOR}$1"
  fi
}

error() {
  if [[ "${LOG_LEVEL}" == "DEBUG" ]] ||\
     [[ "${LOG_LEVEL}" == "INFO" ]] ||\
     [[ "${LOG_LEVEL}" == "WARN" ]] ||\
     [[ "${LOG_LEVEL}" == "ERROR" ]]; then
    local log_title
    if [ -n "${LOG_TITLE}" ]; then
     log_title="(${LOG_TITLE})"
    else
     log_title=''
    fi
    echo -e "${RED}[ERROR]${log_title} ${NO_COLOR}$1"
  fi
}

######################################################
# Check logics
######################################################
set_packages_and_check_cmd() {
  case $OS in
  *"debian"* | *"ubuntu"* )
    CHECK_CMD='dpkg -l | grep -w'
    PACKAGES=(nfs-common open-iscsi)
    ;;
  *"centos"* | *"fedora"* | *"rocky"* | *"ol"* )
    CHECK_CMD='rpm -q'
    PACKAGES=(nfs-utils iscsi-initiator-utils)
    ;;
  *"suse"* )
    CHECK_CMD='rpm -q'
    PACKAGES=(nfs-client open-iscsi)
    ;;
  *"arch"* )
    CHECK_CMD='pacman -Q'
    PACKAGES=(nfs-utils open-iscsi)
    ;;
  *"gentoo"* )
    CHECK_CMD='qlist -I'
    PACKAGES=(net-fs/nfs-utils sys-block/open-iscsi)
    ;;
  *)
    CHECK_CMD=''
    PACKAGES=()
    warn "Stop the environment check because '$OS' is not supported in the environment check script."
    exit 1
    ;;
   esac
}

detect_node_kernel_release() {
  local pod="$1"

  KERNEL_RELEASE=$(qcadmin exp kubectl exec -i $pod -- nsenter --mount=/proc/1/ns/mnt -- bash -c 'uname -r')
  echo "$KERNEL_RELEASE"
}

detect_node_os() {
  local pod="$1"

  OS=$(qcadmin exp kubectl exec -i $pod -- nsenter --mount=/proc/1/ns/mnt -- bash -c 'grep -E "^ID_LIKE=" /etc/os-release | cut -d= -f2')
  if [[ -z "${OS}" ]]; then
    OS=$(qcadmin exp kubectl exec -i $pod -- nsenter --mount=/proc/1/ns/mnt -- bash -c 'grep -E "^ID=" /etc/os-release | cut -d= -f2')
  fi
  echo "$OS"
}

check_local_dependencies() {
  local targets=($@)

  local all_found=true
  for ((i=0; i<${#targets[@]}; i++)); do
    local target=${targets[$i]}
    if [ "$(which $target)" = "" ]; then
      all_found=false
      error "Not found: $target"
    fi
  done

  if [ "$all_found" = "false" ]; then
    msg="Please install missing dependencies: ${targets[@]}."
    info "$msg"
    exit 2
  fi

  msg="Required dependencies '${targets[@]}' are installed."
  info "$msg"
}

create_ds() {
cat <<EOF > $TEMP_DIR/environment_check.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: longhorn-environment-check
  name: longhorn-environment-check
spec:
  selector:
    matchLabels:
      app: longhorn-environment-check
  template:
    metadata:
      labels:
        app: longhorn-environment-check
    spec:
      hostPID: true
      containers:
      - name: longhorn-environment-check
        image: ccr.ccs.tencentyun.com/easysoft/alpine:3.12
        args: ["/bin/sh", "-c", "sleep 1000000000"]
        volumeMounts:
        - name: mountpoint
          mountPath: /tmp/longhorn-environment-check
          mountPropagation: Bidirectional
        securityContext:
          privileged: true
      volumes:
      - name: mountpoint
        hostPath:
            path: /tmp/longhorn-environment-check
EOF
  qcadmin exp kubectl create -f $TEMP_DIR/environment_check.yaml > /dev/null
}

cleanup() {
  info "Cleaning up longhorn-environment-check pods..."
  qcadmin exp kubectl delete -f $TEMP_DIR/environment_check.yaml > /dev/null
  rm -rf $TEMP_DIR
  info "Cleanup completed."
}

wait_ds_ready() {
  while true; do
    local ds=$(qcadmin exp kubectl get ds/longhorn-environment-check -o json)
    local numberReady=$(echo $ds | jq .status.numberReady)
    local desiredNumberScheduled=$(echo $ds | jq .status.desiredNumberScheduled)

    if [ "$desiredNumberScheduled" = "$numberReady" ] && [ "$desiredNumberScheduled" != "0" ]; then
      info "All longhorn-environment-check pods are ready ($numberReady/$desiredNumberScheduled)."
      return
    fi

    info "Waiting for longhorn-environment-check pods to become ready ($numberReady/$desiredNumberScheduled)..."
    sleep 3
  done
}

check_mount_propagation() {
  local allSupported=true
  local pods=$(qcadmin exp kubectl -l app=longhorn-environment-check get po -o json)

  local ds=$(qcadmin exp kubectl get ds/longhorn-environment-check -o json)
  local desiredNumberScheduled=$(echo $ds | jq .status.desiredNumberScheduled)

  for ((i=0; i<desiredNumberScheduled; i++)); do
    local pod=$(echo $pods | jq .items[$i])
    local nodeName=$(echo $pod | jq -r .spec.nodeName)
    local mountPropagation=$(echo $pod | jq -r '.spec.containers[0].volumeMounts[] | select(.name=="mountpoint") | .mountPropagation')

    if [ "$mountPropagation" != "Bidirectional" ]; then
      allSupported=false
      error "node $nodeName: MountPropagation is disabled"
    fi
  done

  if [ "$allSupported" != "true" ]; then
    error "MountPropagation is disabled on at least one node. As a result, CSI driver and Base image cannot be supported"
    exit 1
  else
    info "MountPropagation is enabled"
  fi
}

check_hostname_uniqueness() {
  hostnames=$(qcadmin exp kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="Hostname")].address}')

  deduplicate_hostnames=()
  num_nodes=0
  for hostname in ${hostnames}; do
    num_nodes=$((num_nodes+1))
    if ! echo "${deduplicate_hostnames[@]}" | grep -q "\<${hostname}\>"; then
      deduplicate_hostnames+=("${hostname}")
    fi
  done

  if [ "${#deduplicate_hostnames[@]}" != "${num_nodes}" ]; then
    error "Nodes do not have unique hostnames."
    exit 2
  fi

  info "All nodes have unique hostnames."
}

check_nodes() {
  local name=$1
  local callback=$2
  shift
  shift

  info "Checking $name..."

  local all_passed=true

  local pods=$(qcadmin exp kubectl get pods -o name -l app=longhorn-environment-check)
  for pod in ${pods}; do
    eval "${callback} ${pod} $@"
    if [ $? -ne 0 ]; then
      all_passed=false
    fi
  done

  if [ "$all_passed" = "false" ]; then
    return 1
  fi
}

check_iscsid() {
  local pod=$1

  qcadmin exp kubectl exec -t ${pod} -- nsenter --mount=/proc/1/ns/mnt -- bash -c "systemctl status --no-pager iscsid.service" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    qcadmin exp kubectl exec -t ${pod} -- nsenter --mount=/proc/1/ns/mnt -- bash -c "systemctl status --no-pager iscsid.socket" > /dev/null 2>&1
      if [ $? -ne 0 ]; then
      node=$(qcadmin exp kubectl get ${pod} --no-headers -o=custom-columns=:.spec.nodeName)
      error "Neither iscsid.service nor iscsid.socket is not running on ${node}"
      return 1
    fi
  fi
}

check_multipathd() {
  local pod=$1

  qcadmin exp kubectl exec -t $pod -- nsenter --mount=/proc/1/ns/mnt -- bash -c "systemctl status --no-pager multipathd.service" > /dev/null 2>&1
  if [ $? = 0 ]; then
    node=$(qcadmin exp kubectl get ${pod} --no-headers -o=custom-columns=:.spec.nodeName)
    warn "multipathd is running on ${node}"
    return 1
  fi
}

check_packages() {
  local pod=$1

  OS=$(detect_node_os ${pod})
  if [ x"$OS" = x"" ]; then
    error "Failed to detect OS on node ${node}"
    return 1
  fi

  set_packages_and_check_cmd

  for ((i=0; i<${#PACKAGES[@]}; i++)); do
    check_package ${PACKAGES[$i]}
    if [ $? -ne 0 ]; then
      return 1
    fi
  done
}

check_package() {
  local package=$1

  qcadmin exp kubectl exec -i $pod -- nsenter --mount=/proc/1/ns/mnt -- timeout 30 bash -c "$CHECK_CMD $package" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    node=$(qcadmin exp kubectl get ${pod} --no-headers -o=custom-columns=:.spec.nodeName)
    error "$package is not found in $node."
    return 1
  fi
}

check_nfs_client() {
  local pod=$1
  local node=$(qcadmin exp kubectl get ${pod} --no-headers -o=custom-columns=:.spec.nodeName)

  local options=("CONFIG_NFS_V4_2"  "CONFIG_NFS_V4_1" "CONFIG_NFS_V4")

  local kernel=$(detect_node_kernel_release ${pod})
  if [ "x${kernel}" = "x" ]; then
    warn "Failed to check NFS client installation, because unable to detect kernel release on node ${node}"
    return 1
  fi

  for option in "${options[@]}"; do
    qcadmin exp kubectl exec -t ${pod} -- nsenter --mount=/proc/1/ns/mnt -- bash -c "[ -f /boot/config-${kernel} ]" > /dev/null 2>&1
    if [ $? -ne 0 ]; then
      warn "Failed to check $option on node ${node}, because /boot/config-${kernel} does not exist on node ${node}"
      continue
    fi

    check_kernel_module ${pod} ${option} nfs
    if [ $? = 0 ]; then
      return 0
    fi
  done

  error "NFS clients ${options[*]} should be enabled at least one."
  return 1
}

check_kernel_module() {
  local pod=$1
  local option=$2
  local module=$3

  local kernel=$(detect_node_kernel_release ${pod})
  if [ "x${kernel}" = "x" ]; then
    warn "Failed to check kernel config option ${option}, because unable to detect kernel release on node ${node}"
    return 1
  fi

  qcadmin exp kubectl exec -t ${pod} -- nsenter --mount=/proc/1/ns/mnt -- bash -c "[ -e /boot/config-${kernel} ]" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    warn "Failed to check kernel config option ${option}, because /boot/config-${kernel} does not exist on node ${node}"
    return 1
  fi

  value=$(qcadmin exp kubectl exec -t ${pod} -- nsenter --mount=/proc/1/ns/mnt -- bash -c "grep "^$option=" /boot/config-${kernel} | cut -d= -f2")
  if [ -z "${value}" ]; then
    error "Failed to find kernel config $option on node ${node}"
    return 1
  elif [ "${value}" = "m" ]; then
    qcadmin exp kubectl exec -t ${pod} -- nsenter --mount=/proc/1/ns/mnt -- bash -c "lsmod | grep ${module}" > /dev/null 2>&1
    if [ $? -ne 0 ]; then
      node=$(qcadmin exp kubectl get ${pod} --no-headers -o=custom-columns=:.spec.nodeName)
      error "kernel module ${module} is not enabled on ${node}"
      return 1
    fi
  elif [ "${value}" = "y" ]; then
    return 0
  else
    warn "Unknown value for $option: $value"
    return 1
  fi
}

check_hugepage() {
  local pod=$1
  local expected_nr_hugepages=$2

  nr_hugepages=$(qcadmin exp kubectl exec -i ${pod} -- nsenter --mount=/proc/1/ns/mnt -- bash -c 'cat /proc/sys/vm/nr_hugepages')
  if [ $? -ne 0 ]; then
    error "Failed to check hugepage size on node ${node}"
    return 1
  fi

  if [ $nr_hugepages -lt $expected_nr_hugepages ]; then
    error "Hugepage size is not enough on node ${node}. Expected: ${expected_nr_hugepages}, Actual: ${nr_hugepages}"
    return 1
  fi
}

function check_nvme_cli() {
  local pod=$1

  value=$(qcadmin exp kubectl exec -i $pod -- nsenter --mount=/proc/1/ns/mnt -- bash -c 'nvme version' 2>/dev/null)
  if [ $? -ne 0 ]; then
    node=$(qcadmin exp kubectl get ${pod} --no-headers -o=custom-columns=:.spec.nodeName)
    error "Failed to check nvme-cli version on node ${node}"
    return 1
  fi

  local actual_version=$(echo "$value" | grep -o "[0-9]\+\.[0-9]\+")
  if [[ "$(printf '%s\n' "${NVME_CLI_VERSION}" "$actual_version" | sort -V | tail -n1)" == "$actual_version" ]]; then
    return 0
  fi
  error "nvme-cli version should be at least ${NVME_CLI_VERSION} on node ${node}. Actual: ${actual_version}"
  return 1
}

function check_sse42_support() {
  local pod=$1

  node=$(qcadmin exp kubectl get ${pod} --no-headers -o=custom-columns=:.spec.nodeName)

  machine=$(qcadmin exp kubectl exec -i $pod -- nsenter --mount=/proc/1/ns/mnt -- bash -c 'uname -m' 2>/dev/null)
  if [ $? -ne 0 ]; then
    error "Failed to check machine on node ${node}"
    return 1
  fi

  if [ "$machine" = "x86_64" ]; then
    sse42_support=$(qcadmin exp kubectl exec -i $pod -- nsenter --mount=/proc/1/ns/mnt -- bash -c 'grep -o sse4_2 /proc/cpuinfo | wc -l' 2>/dev/null)
    if [ $? -ne 0 ]; then
      error "Failed to check SSE4.2 instruction set on node ${node}"
      return 1
    fi

    if [ "$sse42_support" -ge 1 ]; then
      return 0
    fi

    error "CPU does not support SSE4.2"
    return 1
  else
    warn "Skip SSE4.2 instruction set check on node ${node} because it is not x86_64"
  fi
}

function show_help() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
    -s, --enable-spdk           Enable checking SPDK prerequisites
    -p, --expected-nr-hugepages Expected number of 2 MiB hugepages for SPDK. Default: 512
    -h, --help                  Show this help message and exit
EOF
    exit 0
}

enable_spdk=false
expected_nr_hugepages=512
while [[ $# -gt 0 ]]; do
    opt="$1"
    case $opt in
        -s|--enable-spdk)
            enable_spdk=true
            ;;
        -p|--expected-nr-hugepages)
            expected_nr_hugepages="$2"
            shift
            ;;
        -h|--help)
            show_help
            ;;
        *)
            instance_manager_options+=("$1")
            ;;
    esac
    shift
done

######################################################
# Main logics
######################################################
DEPENDENCIES=("qcadmin" "jq" "mktemp")
check_local_dependencies "${DEPENDENCIES[@]}"

# Check the each host has a unique hostname (for RWX volume)
check_hostname_uniqueness

# Create a daemonset for checking the requirements in each node
TEMP_DIR=$(mktemp -d)

trap cleanup EXIT
create_ds
wait_ds_ready

check_mount_propagation
check_nodes "iscsid" check_iscsid
check_nodes "multipathd" check_multipathd
check_nodes "packages" check_packages
check_nodes "nfs client" check_nfs_client

if [ "$enable_spdk" = "true" ]; then
  check_nodes "x86-64 SSE4.2 instruction set" check_sse42_support
  check_nodes "nvme-cli" check_nvme_cli
  check_nodes "kernel module nvme_tcp" check_kernel_module CONFIG_NVME_TCP nvme_tcp
  check_nodes "kernel module uio_pci_generic" check_kernel_module CONFIG_UIO_PCI_GENERIC uio_pci_generic
  check_nodes "hugepage" check_hugepage ${expected_nr_hugepages}
fi

exit 0
