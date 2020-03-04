# checkers

## network

`checkNetwork.go`

### network检测项

- 基本信息 `basic`
  - 网卡、ip、路由 `net`
  - bond状态 `bonding.stats`
  - /etc/resolv.conf `resolv.conf`
  - /etc/hosts `hosts.concerned`
  - 内核参数 `kernel.runtime.parameters`

### network配置项（具体的值通过--conf指定的yaml文件配置）

```yaml
checkInterval: 1m0s # 检测间隔，缺省为1m
etc.hosts.concerned: # 关注的hostname列表，缺省为空
- host1
- host2
etc.hosts.path: /host/etc/hosts # 缺省为{mount_point}/etc/hosts
etc.resolv.conf.path: /host/etc/resolv.conf # 缺省为{mount_point}/etc/resolv.conf
kernel.parameters: # 关注的内核参数列表，缺省为空
- net.bridge.bridge-nf-call-iptables
- net.bridge.bridge-nf-call-ip6tables
- net.ipv4.ip_local_reserved_ports
- net.ipv4.ip_forward
net.route.path: /host/proc/net/route # 缺省为{mount_path}/proc/net/route
status.file.path: /host/sys/class/net # 缺省为{mount_path}/sys/class/net
```

## os

`checkOS.go`

### os检测项

- 基本信息 `basic`
  - 主机名 `hostname`
  - 负载 `loads`
  - cpu状态 `stats`
  - os版本、内核版本 `uname`
  - systemd服务 `units`
  - 内核参数 `kernel.runtime.parameters`

### os配置项（具体的值通过--conf指定的yaml文件配置）

```yaml
checkInterval: 1m0s # 检测间隔，缺省为1m
kernel.parameters: # 关注的内核参数列表，缺省为空
- vm.dirty_background_ratio
- vm.dirty_ratio
- vm.max_map_count
units: # 关注的systemd服务，缺省为空
- network.service
- kubelet.service
- ambari-agent.service
- firewalld.service
- ntpd.service
- docker.service
- etcd.service
```

## kubernetes

`checkKubernetes.go`

### kubernetes检测项

- 基本信息 `basic`
  - kubelet.conf中的client信息 `clientConfigAuth`
  - docker信息，版本、容器数量、配置等 `docker`
  - kubernetes信息 `kubernetes`
    - 本节点信息 `local`
    - 命名空间 `namespaces`
    - 节点列表，pod数量，到其它节点的pod网络是否可达 `nodes`
- 详情 `detail`
  - 本节点在kubernetes中的详细情况 `localNode`
  - 容器 `docker.containers`
  - 镜像 `docker.images`
  - docker信息 `docker.info`

### kubernetes配置项（具体的值通过--conf指定的yaml文件配置）

```yaml
checkInterval: 1m0s # 检测间隔，缺省为2m
dcoker.api.version: "1.22" # docker api的版本，缺省为1.22
docker.host: unix:///host/var/run/docker.sock # 缺省为unix://{mount_point}/var/run/docker.sock
kubelet.conf.path: /host/etc/kubernetes/kubelet.conf # 缺省为{mount_point}/etc/kubernetes/kubelet.conf
pingTimeout: 5s # ping的超时时间，用于判断到各个节点上的pod网络是否连通，缺省为5s
```

## hadoop

`checkHadoop.go`

### hadoop检测项

- 基本信息 `basic`
  - /etc/krb5.conf `krb5`
  
### hadoop配置项（具体的值通过--conf指定的yaml文件配置）

```yaml
checkInterval: 1m0s # 检测间隔，缺省为1m
etc.krb5.conf.path: /host/etc/krb5.conf # 缺省为{mount_point}/etc/krb5.conf
```