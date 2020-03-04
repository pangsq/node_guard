# 设计

## 用途

NodeGuard的功能只有两块，即 "收集"和"展示" 数据。
"收集"模块负责将关注的数据收集并存起来，"展示"负责将已存储的数据展示。

### 收集

- 每个checker负责收集相关数据和做出状态判断，具体收集信息见`checkers.md`
- 分为basic/detail/errors,即基本信息、详细信息、异常信息
- 由内部定时器不断触发check()方法

### 展示

- `/`展示汇总所有checker的基本信息和诊断信息
- `/checker/xxx/detail`展示名为xxx的checker的详细信息
- `/checker/xxx/config`展示名为xxx的checker的配置信息
- 由外部触发

### 考虑

"收集"和"展示"两个功能时序上并无依赖关系的原因是有些数据的收集可能会比较耗时，同时也可以防止外部触发频率过高导致出乎预期的"收集"频率过高。

## 配置

为了满足可扩展性，所有的配置项均有外部入口，例如每个checker的checkerInterval（即"收集"操作的间隔），和一些全局的配置。

### 全局的配置

`checkers` checkers总配置项，在配置文件(-c指定的yaml文件)中配置。例子如下，#后有解释

```yaml
checkers:
  - disable: # 禁用的checker
    - checker1
    - checker2
    - ...
```

`daemon` 配置, 通过命令的参数配置，以及连带计算出的一些额外的供checkers公用的配置项。例子如下，#后有解释，具体怎么用的搜代码吧。

```yaml
daemon:
  dbus_address: unix:path=/host/run/systemd/private # 依赖mount_point
  debug_enable: true # debug模式是否开启，通过参数-d配置
  mount_point: /host # 挂载点，相当重要，通过参数-m配置
  proc_path: /host/proc # 依赖mount_point
  rootfs_path: /host # 依赖mount_point
  separator: ; # 如果checker的配置是env传入的，并且是数组，那么这里的符号即是分隔符
  sys_path: /host/sys # 依赖mount_point
```

### checker的配置

checker的具体配置见 checkers.md。

## 路由

### 输出格式

通过在url中加上`?format=`选择展示的数据格式，目前支持yaml、json，缺省为yaml。

后续如果有机会的话，可以考虑下支持更多的展示格式。

### 全局的路由

- `/` 所有checker收集的基本数据，包含basic和errors两项
- `/configs` 配置项，包含全局配置和每个checker的配置

### pprof的路由

参考docker的做法，直接将golang的"net/http/pprof"暴露出来了

```golang
func profilerSetup(r *mux.Router) {
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	r.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	r.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	r.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
}
```

### checker的路由

每个checker下会默认带一个/config的路由，例如对于os这个checker来说完整的config路由为/os/config，当然这个config的内容已经被包含在/configs这个路由展示的内容中了。

checker还可以暴露其他的路由，如果有需要的话，见 checkers.md

## 代码结构

`guard.go` 是入口，包含main函数。

`server.go` 是路由层，可以理解成通俗的controller层。

`daemon.go` 是核心层，也可以理解成service层。

`daemonConfig.go` 是配置处理部分，`getOrDefault`既支持从配置文件读取值，也支持从env读取，也可以选择内置于代码中的默认值。

`checker.go` checker接口，定义了一个checker应该有的操作，提供了checker自注册的方法。

`checkerXxxx.go` 每个checker的具体逻辑。大致是 "被启动之后，通过一个计时器不断触发check()收集数据，并对外提供这些数据"。

`utils.go` 公用方法。