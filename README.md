# Node Guard

参考[node_exporter](https://github.com/prometheus/node_exporter)的做法，提供一个监控K8S集群节点情况(OS内核参数、网络连通性、Hadoop相关的配置等等)的程序。



## 编译

```shell
dep ensure
CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' .
```

### 调试

为了排查是否出现竞态问题，可以在编译时加上--race

kubernetes的访问需要可用的kubelet.conf，通过`kubelet.conf.path`指定kubelet.conf的路径

docker的访问可以开放docker daemon的监听端口，通过`docker.host`指向确切的地址

## 构造镜像

```shell
docker build -t node-guard:1.0
```

## 使用

```shell
Usage of ./node_guard:
  -c string
    	the config file path
  -d	debug mode
  -m string
    	mount point
  -p int
    	listen port (default 8080)
```

### 以容器方式运行

为了收集宿主机的状态信息，所以需要将宿主机的整个根路径挂载到容器内。注意-m选择宿主机的挂载点。

举个栗子:

#### 1. 创建一个容器

```shell
docker run -tid --entrypoint /bin/sh --name node_guard -v /:/host --net host --memory=128M --cpu-period=100000 --cpu-quota=1000 node_guard:1.0
```

选择将宿主机的根路径挂在到了容器内的/host下

#### 2. 在容器中运行NodeGuard

```shell
./node_guard -c conf.yaml -p 8080 -m /host -d 
```

-m指定了/host

### 在kubernetes中运行

参考node_guard.yaml就可以了

## 想要了解更多？

请参考

[docs/checkers.md](docs/checkers.md)

[docs/design.md](docs/design.md)