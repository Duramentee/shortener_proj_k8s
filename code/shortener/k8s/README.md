# k8s · 集群部署指引（阶段 4 至阶段 6）

> 建立日期：2026-10-01。
> 这份文件与 `code/shortener/README.md` 第 7 节的阶段 4、阶段 5、阶段 6 一一对应。
> 清单目录中的纯净版可直接应用，`templates/` 中的带注释版本用于理解字段与排障；两者描述相同的对象。

---

## 0. 这份指引怎么使用

### 0.1 分工与最终产物

| 参与方 | 负责的内容 | 最终产物 |
|---|---|---|
| 你 | 执行验收命令，记录实测现象，并根据实际集群环境调整资源取值 | 一份现象记录与可复现的部署结果 |
| 清单 | `k8s/` 顶层保存可直接应用的纯净版，`k8s/templates/` 保存逐字段解释的注释版 | 六个组件清单及对应模板 |

### 0.2 前置条件自检

在动手写清单之前，先执行下表四条命令，四条全部通过之后再继续；任何一条不通过时，先解决它，否则后面的报错会同时包含「环境问题」与「清单问题」两层原因，排查难度会显著上升。

| 序号 | 检查项 | 命令 | 通过标准 | 不通过时的处理方式 |
|---|---|---|---|---|
| 1 | Docker 引擎可用 | `docker ps` | 输出表头 `CONTAINER ID IMAGE ...`，并且能够列出正在运行的容器 | 在 Windows 侧启动 Docker Desktop，并且在 Settings → Resources → WSL Integration 中打开当前发行版的开关；随后重新执行 `docker ps`。当前的失败形态是输出 `The command 'docker' could not be found in this WSL 2 distro.`，这条消息说明 WSL 集成没有生效，而不是说明本机没有安装 Docker |
| 2 | kind 可用 | `kind version` | 输出版本号，本机当前是 `v0.33.0` | 本机的 kind 位于 `/home/drow/go/bin/kind`，确认 `$PATH` 中包含 `/home/drow/go/bin` |
| 3 | kubectl 可用 | `kubectl version --client` | 输出客户端版本号 | 本机当前缺失，按第 0.3 节安装 |
| 4 | 集群是否已经存在 | `kind get clusters` | 输出 `shortener` | 集群不存在时按第 0.4 节创建 |

### 0.3 安装 kubectl（本机当前缺失）

本机由 Go 工具的安装方式统一管理，因此推荐用 `go install` 安装，前提是 `/home/drow/go/bin` 已经位于 `$PATH` 中（kind 就在这个目录下并且可以调用，说明该目录已经在 `$PATH` 中）。

```bash
GOPROXY=https://goproxy.cn,direct go install k8s.io/kubectl@v1.33.0
kubectl version --client
```

必须带 `GOPROXY` 的原因与后端构建时相同：`proxy.golang.org` 在这台机器上不可达，不带这个环境变量时模块下载会直接失败。

备选方式是直接下载官方二进制文件，该方式需要你自己在终端里执行并且需要 `sudo` 权限：

```bash
curl -LO "https://dl.k8s.io/release/v1.33.0/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
```

版本对齐要求：kubectl 与集群 API 服务器的主版本号相差不能超过一个小版本。集群创建之后执行 `kubectl version` 查看 `Server Version`，如果与客户端相差超过一个小版本，再安装与服务端匹配的版本。

### 0.4 创建集群与导入镜像

```bash
# 1. 创建集群，节点是 Docker 容器
kind create cluster --name shortener

# 2. 构建两个业务镜像，镜像名的后缀 dev 表示这是本地开发版本
docker build -t shortener-api:dev ./code/shortener/api
docker build -t shortener-web:dev ./code/shortener/web

# 3. 把两个镜像加载进集群的节点
kind load docker-image shortener-api:dev shortener-web:dev --name shortener
```

第 3 步是本阶段最容易遗漏的一步，它必须存在的原因如下：kind 的节点本身就是 Docker 容器，节点容器内部的容器运行时（containerd）看不到宿主机 Docker 的镜像库，因此用 `docker build` 构建出来的镜像在集群里并不存在，Pod 会以 `ErrImageNeverPull` 或者 `ImagePullBackOff` 的状态停在原地。`kind load docker-image` 的动作就是把镜像从宿主机 Docker 的镜像库搬迁到节点容器的 containerd 镜像库中。

对应地，清单中的 `imagePullPolicy` 必须写 `IfNotPresent`。当标签不是 `latest` 时，Kubernetes 的默认策略本来就是 `IfNotPresent`，但当标签是 `latest` 时默认策略会变成 `Always`，此时即使镜像已经加载进节点，kubelet 仍然会去远端仓库拉取，并且因为仓库里没有这个镜像而失败。把策略显式写出来可以避免这个隐式规则带来的问题。

### 0.5 文件清单与编写顺序

按编号顺序创建文件，这样 `kubectl apply -f code/shortener/k8s/` 这条命令会按文件名排序依次执行，而被引用的对象（命名空间、ConfigMap、Secret、Service）总是先于引用它的对象（StatefulSet、Deployment）被创建。

顶层目录与模板目录的分工如下表。这份分工存在的必要性是：`kubectl apply -f <目录>` 在不加 `-R` 参数时不会递归进入子目录，因此存放在 `templates/` 中的模板文件不会被应用；如果模板与纯净版放在同一层目录中，同名的对象会被应用两次，而模板中留空的取值会使校验失败。

| 目录 | 存放的内容 | 是否参与 `kubectl apply -f code/shortener/k8s/` |
|---|---|---|
| `k8s/` 顶层 | 不带注释、取值已经填写的纯净版清单 | 参与，目录中每个 `.yaml` 文件都会按文件名顺序被应用 |
| `k8s/templates/` | 带完整注释的模板，其中每个键都写明取值、设置理由与写错的后果 | 不参与，因为该命令不递归子目录；需要单独应用模板时要在命令中显式写出模板的路径 |

| 序号 | 文件名 | 创建的对象 | 依赖的前序文件 | 完成标志 |
|---|---|---|---|---|
| 1 | `00-namespace.yaml` | Namespace `shortener` | 无 | `kubectl get ns shortener` 的输出中状态是 `Active` |
| 2 | `10-config.yaml` | ConfigMap `api-config`、Secret `postgres-secret` | `00-namespace.yaml` | 两条 `kubectl get` 命令都能取到对象，并且键的数量正确 |
| 3 | `20-postgres.yaml` | Service `postgres`（Headless）、StatefulSet `postgres` | `10-config.yaml` | `postgres-0` 处于 `Running` 且 `READY` 列是 `1/1`，PVC 的状态是 `Bound` |
| 4 | `30-redis.yaml` | Service `redis`（ClusterIP）、Deployment `redis` | `00-namespace.yaml` | `redis` 的 Pod 处于 `Running` 且 `READY` 列是 `1/1` |
| 5 | `40-api.yaml` | Service `api`（ClusterIP）、Deployment `api` | `10-config.yaml`、`20-postgres.yaml`、`30-redis.yaml` | 两个 `api` Pod 都处于 `Running` 且 `READY` 列是 `1/1`，`kubectl get endpoints api -n shortener` 中有两个地址 |
| 6 | `50-web.yaml` | Service `web`（NodePort）、Deployment `web` | `40-api.yaml` | 两个 `web` Pod 都处于 `Running` 且 `READY` 列是 `1/1` |

### 0.6 全局命名与端口约定

下表中的取值不是风格偏好，而是被其他组件硬性引用的事实，改动其中任何一项都必须同步修改引用方，否则会出现「一个组件正常、另一个组件启动即失败」的现象。

| 项目 | 必须使用的取值 | 必须使用这个取值的原因 |
|---|---|---|
| 命名空间的名称 | `shortener` | 全部清单的 `metadata.namespace` 字段都引用它，删除命名空间会级联删除其中全部对象 |
| 后端 Service 的名称 | `api` | `web/nginx.conf` 中的 `upstream` 写的是 `server api:8080;`，Service 名称不一致时 nginx 启动阶段就报 `host not found in upstream "api"` 并且容器反复重启 |
| 后端 Service 的端口 | `8080` | 与 `upstream` 后面的端口号一致 |
| 数据库 Service 的名称 | `postgres` | Secret 中的 `DATABASE_URL` 主机名必须写 `postgres`，写成 `localhost` 时后端容器会尝试连接它自己 |
| 数据库 Service 的端口 | `5432` | 与 `DATABASE_URL` 中的端口一致 |
| 缓存 Service 的名称 | `redis` | ConfigMap 中的 `REDIS_ADDR` 取值必须是 `redis:6379` |
| 前端容器的端口 | `80` | `web/Dockerfile` 中的 `EXPOSE 80` 与 `nginx.conf` 中的 `listen 80` 共同决定 |
| 前端 Service 的类型与端口 | `NodePort`，`nodePort: 30080` | `code/shortener/README.md` 第 2 节约定宿主机端口为 30080，与 Compose 形态的 `8080:80` 是同一件事的两种实现 |
| 两个镜像的名称 | `shortener-api:dev`、`shortener-web:dev` | 与第 0.4 节的构建命令一致，`kind load docker-image` 加载的就是这两个标签 |
| 标签选择器 | 统一使用 `app: api`、`app: web`、`app: postgres`、`app: redis` 这一组 | 每条 `kubectl get po -l app=api -n shortener` 与 `kubectl describe po -l app=api -n shortener` 命令都依赖它，标签与选择器不一致时 Service 的 Endpoints 会是空的 |

---

## 1. 第 1 步 命名空间

文件：`00-namespace.yaml`。

| 字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| `apiVersion` | 必须 | `v1` |
| `kind` | 必须 | `Namespace` |
| `metadata.name` | 必须 | `shortener` |
| `metadata.labels` | 可选 | 例如 `app.kubernetes.io/part-of: shortener`，只用于分类检索 |

需要理解的机制：Namespace 是集群级别的对象，它的作用是把对象名称的作用域分隔开，并且为资源配额与访问控制提供边界。后续五个文件中的每一个对象都必须在 `metadata.namespace` 中写 `shortener`，或者在执行 `kubectl apply` 时加 `-n shortener` 参数；建议在清单里显式写出来，因为清单是自描述的，而命令行的参数在别人复现时容易遗漏。

另外要记住删除的后果：`kubectl delete ns shortener` 会级联删除命名空间内的全部对象，包括 StatefulSet 生成的 PVC（取决于回收策略，`persistentVolumeReclaimPolicy` 与 StorageClass 的配置共同决定 PVC 删除之后数据卷是否一并删除）。这条命令可以用于实验结束后清理环境。

---

## 2. 第 2 步 配置注入（ConfigMap 与 Secret）

文件：`10-config.yaml`，里面写两个对象。

### 2.1 与 `compose.yaml` 的逐项对应关系

后端读取配置的方式只有一个来源，就是环境变量，因此 Compose 的 `environment` 与 Kubernetes 的 ConfigMap 与 Secret 是同一件事在两种部署方式下的实现。下表逐项列出对应关系、取值以及放置理由。

| 环境变量 | 取值 | 放置位置 | 放置理由 |
|---|---|---|---|
| `DATABASE_URL` | `postgres://shortener:shortener_dev_password@postgres:5432/shortener?sslmode=disable` | Secret | 这个取值中包含数据库口令，凡是包含口令的取值都必须放进 Secret |
| `REDIS_ADDR` | `redis:6379` | ConfigMap | 这个取值不含敏感信息，主机名取自同命名空间内的 Service 名称 |
| `REDIS_PASSWORD` | 空字符串 | 不设置 | 本地联调的 Redis 没有配置口令，后端 `config.Load` 的默认值就是空字符串 |
| `REDIS_DB` | `0` | 不设置 | 与 `config.Load` 的默认值一致，不需要显式设置 |
| `CACHE_TTL` | `300s` | ConfigMap | 与默认值一致，显式写出来是为了让「缓存生存时间是 300 秒」这件事在集群清单中可见 |
| `CLICK_FLUSH_INTERVAL` | `5s` | ConfigMap | 阶段 6 需要把它改成 `60s` 来观察写回周期的效果，因此必须做成可修改的字段 |
| `SHUTDOWN_TIMEOUT` | `10s` | ConfigMap | 它决定收到停止信号之后等待写回完成的时限，与 Pod 的 `terminationGracePeriodSeconds` 有直接关系 |
| `GIN_MODE` | `release` | ConfigMap | 集群环境中不需要 gin 的路由清单输出，与 `api/Dockerfile` 中的默认取值保持一致 |
| `POSTGRES_USER` | `shortener` | Secret | 数据库初始化参数，与 `DATABASE_URL` 中的用户名必须一致 |
| `POSTGRES_PASSWORD` | `shortener_dev_password` | Secret | 数据库初始化参数，与 `DATABASE_URL` 中的口令必须一致 |
| `POSTGRES_DB` | `shortener` | Secret | 数据库初始化参数，与 `DATABASE_URL` 中的库名必须一致 |
| `POSTGRES_INITDB_ARGS` | `--encoding=UTF8 --locale=C` | ConfigMap | 与 `compose.yaml` 中保持一致，避免不同机器上排序规则不一致 |

### 2.2 ConfigMap 的字段要求

| 字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| `apiVersion` | 必须 | `v1` |
| `kind` | 必须 | `ConfigMap` |
| `metadata.name` | 必须 | `api-config` |
| `metadata.namespace` | 必须 | `shortener` |
| `data` | 必须 | 键值对形式，键是环境变量名称，值必须是字符串；上表中「放置位置是 ConfigMap」的条目全部写在这里 |

### 2.3 Secret 的字段要求

| 字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| `apiVersion` | 必须 | `v1` |
| `kind` | 必须 | `Secret` |
| `metadata.name` | 必须 | `postgres-secret` |
| `type` | 可选 | 不写时默认是 `Opaque`，本项目的取值不属于内置类型，因此可以省略或显式写 `Opaque` |
| `stringData` | 二选一 | 这里写明文，API 服务器在写入 etcd 之前会自动做 base64 编码；这是推荐写法，因为不需要手动编码 |
| `data` | 二选一 | 这里写 base64 编码之后的取值，需要自己执行 `echo -n 'xxx' \| base64` 生成；两处同时出现同一个键时 `stringData` 优先 |

需要理解两件事。第一，Secret 只解决「取值不在常规输出中直接显示」这件事，它本身不是加密机制；决定谁能读取 Secret 的是 RBAC 配置与 etcd 的静态加密设置。第二，`kubectl get secret postgres-secret -n shortener -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d` 这条命令可以取回明文，可以用它验证自己写入的取值是否正确。

### 2.4 把配置注入容器的两种写法

| 写法 | 字段结构 | 适用场景与注意事项 |
|---|---|---|
| 逐个键注入 | `env` 数组中的每一项写 `name`，再加 `valueFrom.configMapKeyRef` 或者 `valueFrom.secretKeyRef`，其中必须同时写出 `name` 与 `key` | 适用于只需要部分键，或者需要把一个键的值改写成另一个环境变量名称的场景 |
| 整体注入 | `envFrom` 数组中的每一项写 `configMapRef` 或者 `secretKeyRef`，只写对象的 `name` | 适用于把 ConfigMap 中的全部键一次性变成环境变量；注意键名必须都是合法的环境变量名称，否则该键会被跳过并在事件中记录错误 |

两条必须记住的时序规则：

第一，引用的键不存在时容器无法创建，Pod 会停在 `CreateContainerConfigError` 状态，因此 ConfigMap 与 Secret 必须先于 Deployment 创建。

第二，用 `env` 方式注入的取值在容器创建时就固化成环境变量，之后修改 ConfigMap 不会影响已经运行的 Pod，必须执行 `kubectl rollout restart deployment/<名称> -n shortener` 触发一次滚动重建才会生效；只有把 ConfigMap 以卷的形式挂载时，文件内容才会自动更新。阶段 6 的配置故障实验正是围绕这条规则设计的。

---

## 3. 第 3 步 PostgreSQL（Headless Service 与 StatefulSet）

文件：`20-postgres.yaml`，里面写 Service 与 StatefulSet 两个对象。

### 3.1 为什么这里必须使用 StatefulSet 而不是 Deployment

| 对比项 | Deployment | StatefulSet | 本项目的结论 |
|---|---|---|---|
| Pod 名称 | 名称由工作负载名称加一段随机后缀组成，例如 `postgres-7d9f5b6c4-x2k9m` | 名称由工作负载名称加序号组成，例如 `postgres-0` | 数据库需要一个稳定的网络标识，因此使用 StatefulSet |
| 重建后的名称 | 重新调度之后名称会改变 | 重建之后名称不变，仍然是 `postgres-0` | 名称不变是「新 Pod 挂载同一个 PVC」这条机制得以成立的前提 |
| 存储申请方式 | 只能在 `template` 中引用一个已经存在的 PVC，因此多个副本会共享同一个卷 | 用 `volumeClaimTemplates` 为每一个序号单独生成一个 PVC，名称形如 `data-postgres-0` | 数据库需要独占存储，因此使用 `volumeClaimTemplates` |
| 启动与停止顺序 | 并行 | 默认按序号顺序启动、逆序停止 | 单副本场景下这条差异不产生可见影响，但它是面试中常被追问的差异点 |

`volumeClaimTemplates` 是 StatefulSet 独有的字段，把它写在 Deployment 的 `template` 中属于非法字段，API 服务器会直接拒绝该清单。

### 3.2 Service 的字段要求

| 字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| `kind` | 必须 | `Service` |
| `metadata.name` | 必须 | `postgres` |
| `spec.clusterIP` | 必须 | `None`。这个取值把 Service 变成 Headless Service，也就是不给它分配虚拟 IP，DNS 查询直接返回后端 Pod 的地址 |
| `spec.selector` | 必须 | 与 StatefulSet 的 `template.metadata.labels` 完全一致，取值建议 `app: postgres` |
| `spec.ports[0].port` | 必须 | `5432`，这是 Service 暴露的端口 |
| `spec.ports[0].targetPort` | 必须 | `5432`，这是容器监听的端口 |
| `spec.ports[0].name` | 建议 | `postgres`，多端口 Service 中端口必须命名 |

Headless Service 与 StatefulSet 的 `spec.serviceName` 字段互相引用：`serviceName` 写 `postgres`，它决定每个 Pod 获得的 DNS 名称的域名部分，形如 `postgres-0.postgres.shortener.svc.cluster.local`。

### 3.3 StatefulSet 的字段要求

| 字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| `metadata.name` | 必须 | `postgres` |
| `spec.serviceName` | 必须 | `postgres`，与上表 Service 的名称一致 |
| `spec.replicas` | 必须 | `1` |
| `spec.selector.matchLabels` | 必须 | `app: postgres`，StatefulSet 的该字段创建之后不可修改 |
| `spec.template.metadata.labels` | 必须 | 与上一条完全一致，否则选择器匹配不到任何 Pod |
| `spec.template.spec.containers[0].name` | 必须 | `postgres` |
| `spec.template.spec.containers[0].image` | 必须 | `postgres:16-alpine`，与 `compose.yaml` 中的镜像保持一致 |
| `spec.template.spec.containers[0].env` | 必须 | `POSTGRES_USER`、`POSTGRES_PASSWORD`、`POSTGRES_DB` 三个变量用 `valueFrom.secretKeyRef` 引用 `postgres-secret`；`POSTGRES_INITDB_ARGS` 用 `valueFrom.configMapKeyRef` 引用 `api-config` |
| `spec.template.spec.containers[0].env[].name` 为 `PGDATA` | 建议 | 取值写 `/var/lib/postgresql/data/pgdata`，把数据文件放进挂载目录下的子目录，理由见第 3.4 节 |
| `spec.template.spec.containers[0].ports[0].containerPort` | 必须 | `5432` |
| `spec.template.spec.containers[0].volumeMounts[0]` | 必须 | `name` 写 `data`，`mountPath` 写 `/var/lib/postgresql/data`；`name` 必须与 `volumeClaimTemplates` 中的 `metadata.name` 一致 |
| `spec.template.spec.containers[0].readinessProbe` | 必须 | `exec` 类型，命令为 `pg_isready -U shortener -d shortener`；参数必须是容器内部能够识别的用户名与库名 |
| `spec.template.spec.containers[0].livenessProbe` | 建议 | 与就绪探针相同的命令，但参数更宽松（`periodSeconds` 与 `failureThreshold` 更大），避免数据库处于负载高峰时被误判并重启 |
| `spec.template.spec.containers[0].resources` | 必须 | 建议 `requests` 为 `cpu: 100m`、`memory: 128Mi`，`limits` 为 `cpu: 500m`、`memory: 512Mi` |
| `volumeClaimTemplates[0].metadata.name` | 必须 | `data`，与 `volumeMounts` 中的名称一致 |
| `volumeClaimTemplates[0].spec.accessModes` | 必须 | `ReadWriteOnce` |
| `volumeClaimTemplates[0].spec.resources.requests.storage` | 必须 | `1Gi` |
| `volumeClaimTemplates[0].spec.storageClassName` | 建议不写 | 不写时使用集群的默认 StorageClass；kind 创建集群时自带名为 `standard` 的默认 StorageClass |

### 3.4 三个易错点

| 易错点 | 现象 | 原因与处理方式 |
|---|---|---|
| 数据目录直接挂在卷的根目录 | 日志中出现 `initdb: directory "/var/lib/postgresql/data" exists but is not empty`，或者容器以非零状态退出 | 官方镜像的 `PGDATA` 默认指向 `/var/lib/postgresql/data`，而挂载目录的根目录中可能存在 `lost+found` 一类的系统目录，初始化程序因此认为目录非空而拒绝初始化。处理方式是把 `PGDATA` 设为挂载目录下的子目录，例如 `/var/lib/postgresql/data/pgdata` |
| 探针的初始等待时间过短 | `READY` 列长时间是 `0/1`，事件中反复出现 `Readiness probe failed: ... no response` | 数据库第一次启动需要执行 `initdb` 并且回放日志，耗时为若干秒到十几秒，因此 `initialDelaySeconds` 建议写 `10` 以上，或者增加一个 `startupProbe` 让存活探针在初始化期间不参与判断 |
| 选择器与标签不一致 | `kubectl get endpoints postgres -n shortener` 的输出是空的 | `spec.selector.matchLabels` 与 `spec.template.metadata.labels` 必须逐字对应；两处的键与值只要有一处不同，Endpoints 就不会包含任何地址 |

---

## 4. 第 4 步 Redis（Deployment 与 ClusterIP Service）

文件：`30-redis.yaml`。

| 对象与字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| Deployment 的 `spec.replicas` | 必须 | `1` |
| Deployment 的 `spec.selector.matchLabels` 与 `spec.template.metadata.labels` | 必须 | 两处都写 `app: redis` |
| 容器的 `image` | 必须 | `redis:7-alpine` |
| 容器的 `args` | 建议 | `["redis-server", "--appendonly", "yes"]`。`args` 会追加在镜像的 `ENTRYPOINT` 之后，效果与 `compose.yaml` 中的 `command` 相同；打开 AOF 之后容器重启仍然保留缓存键，这样阶段 6 的排障实验才有一条确定的基线 |
| 容器的 `ports[0].containerPort` | 必须 | `6379` |
| 容器的 `readinessProbe` | 必须 | `exec` 类型，命令为 `redis-cli ping`，返回 `PONG` 时判定为就绪 |
| 容器的 `resources` | 必须 | 建议 `requests` 为 `cpu: 50m`、`memory: 64Mi`，`limits` 为 `cpu: 200m`、`memory: 256Mi` |
| Service 的 `metadata.name` | 必须 | `redis`，因为 ConfigMap 中的 `REDIS_ADDR` 就是 `redis:6379` |
| Service 的 `spec.type` | 必须 | `ClusterIP`，缓存不需要对外暴露 |
| Service 的 `spec.ports[0].port` 与 `targetPort` | 必须 | 两者都写 `6379` |
| Service 的 `spec.selector` | 必须 | `app: redis` |

这里不需要 PersistentVolumeClaim，原因是缓存中的数据都可以从 PostgreSQL 重建，丢失只会导致下一次重定向回落到数据库，属于设计上接受的代价。这一条属于设计取舍，在面试中需要能够说明两种做法的差别。

---

## 5. 第 5 步 后端（Deployment 与 ClusterIP Service）

文件：`40-api.yaml`。本文件的对象最多，也最容易被追问。

| 对象与字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| Deployment 的 `metadata.name` | 必须 | `api` |
| Deployment 的 `spec.replicas` | 必须 | `2`，与本项目「无状态、可横向扩展」的设计一致 |
| Deployment 的 `spec.selector.matchLabels` 与 `spec.template.metadata.labels` | 必须 | 两处都写 `app: api` |
| 容器的 `image` 与 `imagePullPolicy` | 必须 | `shortener-api:dev` 与 `IfNotPresent` |
| 容器的 `env` | 必须 | `DATABASE_URL` 用 `valueFrom.secretKeyRef` 引用 `postgres-secret`；`REDIS_ADDR`、`CACHE_TTL`、`CLICK_FLUSH_INTERVAL`、`SHUTDOWN_TIMEOUT`、`GIN_MODE` 用 `valueFrom.configMapKeyRef` 引用 `api-config` |
| 容器的 `ports[0].containerPort` | 必须 | `8080` |
| 容器的 `readinessProbe` | 必须 | `httpGet` 类型，`path` 写 `/api/readyz`，`port` 写 `8080` |
| 容器的 `livenessProbe` | 必须 | `httpGet` 类型，`path` 写 `/api/healthz`，`port` 写 `8080` |
| 容器的 `resources` | 必须 | 建议 `requests` 为 `cpu: 50m`、`memory: 64Mi`，`limits` 为 `cpu: 500m`、`memory: 256Mi`。阶段 6 的内存实验就是把 `limits.memory` 改成一个小于正常用量的取值 |
| Pod 的 `spec.terminationGracePeriodSeconds` | 必须 | 写 `30`，它必须大于 `SHUTDOWN_TIMEOUT` 的 `10s` |
| Service 的 `metadata.name` | 必须 | `api`，`web/nginx.conf` 中的 `upstream` 引用的就是这个名称 |
| Service 的 `spec.type` | 必须 | `ClusterIP`，外部流量只能经过 `web` 的 nginx 进入 |
| Service 的 `spec.ports[0].port` 与 `targetPort` | 必须 | 两者都写 `8080` |
| Service 的 `spec.selector` | 必须 | `app: api` |

### 5.1 两个探针的分工（本阶段最重要的机制）

| 探针 | 检查路径 | 是否检查外部依赖 | 失败之后的动作 | 为什么这样设计 |
|---|---|---|---|---|
| 就绪探针 | `/api/readyz` | 检查，它必须同时确认 PostgreSQL 与 Redis 可用 | 把该 Pod 从 Service 的 Endpoints 中移除，不再向它转发流量，容器本身继续运行 | 依赖不可用时接收流量只会产生 `503` 响应，把它摘出流量入口可以让可用的副本继续服务 |
| 存活探针 | `/api/healthz` | 不检查，它只证明进程仍然能够响应请求 | 重启容器 | 依赖故障属于外部状态，重启本进程既不能修复数据库，又会在依赖恢复之后造成全部副本同时冷启动；把依赖性检查放进存活探针会引发连锁重启 |

如果读者把就绪检查放进存活探针，会观察到这样的现象：Redis Service 暂时没有后端时，`api` Pod 的 `RESTARTS` 列开始增长，而故障本身并不要求重启任何后端进程。阶段 6 的探针对照实验就是复现这个现象。

`terminationGracePeriodSeconds` 的作用可以用一条机制说明：kubelet 发送 `SIGTERM` 之后开始计时，计时到达该字段的取值时如果容器还没有退出，kubelet 会发送 `SIGKILL`。本项目在收到 `SIGTERM` 之后要用最多 `SHUTDOWN_TIMEOUT` 的 `10s` 完成最后一次点击增量写回，因此宽限期必须大于它，否则写回过程被强制中断，这一轮增量会留在 Redis 中，下次以相同短码触发跳转时可能被重复写回。

---

## 6. 第 6 步 前端（Deployment 与 NodePort Service）

文件：`50-web.yaml`。

| 对象与字段 | 是否必须 | 取值要求与含义 |
|---|---|---|
| Deployment 的 `metadata.name` | 必须 | `web` |
| Deployment 的 `spec.replicas` | 必须 | `2` |
| Deployment 的 `spec.selector.matchLabels` 与 `spec.template.metadata.labels` | 必须 | 两处都写 `app: web` |
| 容器的 `image` 与 `imagePullPolicy` | 必须 | `shortener-web:dev` 与 `IfNotPresent` |
| 容器的 `ports[0].containerPort` | 必须 | `80` |
| 容器的 `readinessProbe` | 必须 | `httpGet` 类型，`path` 写 `/`，`port` 写 `80`，返回 `200` 时判定为就绪 |
| 容器的 `resources` | 必须 | 建议 `requests` 为 `cpu: 20m`、`memory: 32Mi`，`limits` 为 `cpu: 200m`、`memory: 128Mi` |
| Service 的 `metadata.name` | 必须 | `web` |
| Service 的 `spec.type` | 必须 | `NodePort` |
| Service 的 `spec.ports[0].port` | 必须 | `80` |
| Service 的 `spec.ports[0].targetPort` | 必须 | `80` |
| Service 的 `spec.ports[0].nodePort` | 必须 | `30080`，取值必须落在默认范围 `30000-32767` 之内 |
| Service 的 `spec.selector` | 必须 | `app: web` |

### 6.1 在 kind 集群中访问 NodePort 的三种方式

kind 的节点本身是 Docker 容器，NodePort 只会绑定在节点容器的网络命名空间中，因此 Windows 侧的浏览器不能直接访问 `localhost:30080`。下表列出三种可行方式与各自的代价。

| 方式 | 命令 | 代价与适用场景 |
|---|---|---|
| 端口转发 | `kubectl port-forward -n shortener svc/web 8080:80`，随后浏览器访问 `http://localhost:8080` | 需要在终端中保持该命令持续运行；阶段 4 的验收用这一条最省事，它验证的是 Pod 与 Service 是否正常，而不是节点端口是否可达 |
| 在 WSL 内部直接访问节点地址 | 先用 `kubectl get nodes -o wide` 取出 `INTERNAL-IP`，再执行 `curl -i http://<该地址>:30080/` | 只能验证集群内部可达性，Windows 浏览器仍然无法访问；适合作为对比实验，用来证明 NodePort 确实已经在节点上监听 |
| 创建集群时把节点端口映射到宿主机 | 用 `kind create cluster --config kind-config.yaml` 创建集群，配置文件中写 `extraPortMappings`，把宿主机端口 `30080` 映射到节点容器的 `30080`，`protocol` 写 `TCP` | 这是与生产环境最接近的方式，代价是需要删除并重建集群，因此建议在阶段 4 开始时就决定是否使用，避免中途重建导致前面的验证结果失效 |

对应 `extraPortMappings` 的配置片段如下（该文件由你编写，此处只说明必须存在的字段）：

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30080   # 节点容器上的端口，对应 Service 的 nodePort
        hostPort: 30080        # Windows 与 WSL 侧可见的端口
        protocol: TCP
```

---

## 7. 阶段 4 的验收

按顺序执行下表命令，全部符合预期即代表阶段 4 完成。

| 序号 | 动作 | 命令 | 预期结果 |
|---|---|---|---|
| 1 | 按顺序应用清单 | `kubectl apply -f code/shortener/k8s/` | 输出六条以上的 `created` 记录，没有 `error` |
| 2 | 确认命名空间 | `kubectl get ns shortener` | 状态列是 `Active` |
| 3 | 列出全部 Pod | `kubectl get po -n shortener` | 共六个 Pod（`postgres-0` 一个、`redis` 一个、`api` 两个、`web` 两个），全部处于 `Running`，并且 `READY` 列是 `1/1` |
| 4 | 检查端点 | `kubectl get endpoints -n shortener` | `api` 与 `web` 各自有两个地址，`postgres` 有一个地址，`redis` 有一个地址；地址为空说明选择器与标签不匹配，或者就绪探针没有通过 |
| 5 | 检查存储声明 | `kubectl get pvc -n shortener` | `data-postgres-0` 的状态列是 `Bound` |
| 6 | 检查后端就绪端点 | `kubectl exec -n shortener deploy/api -- wget -qO- http://localhost:8080/api/readyz` | 输出 `{"status":"ready","checks":{"postgres":"ok","redis":"ok"}}` |
| 7 | 检查集群内的名称解析 | `kubectl exec -n shortener deploy/web -- wget -qO- http://api:8080/api/healthz` | 输出 `{"status":"ok"}`，这一条同时证明 Service 名称是 `api` 并且 DNS 解析正常 |
| 8 | 检查前端静态文件 | `kubectl exec -n shortener deploy/web -- wget -qO- http://localhost/` | 输出 `index.html` 的内容 |
| 9 | 检查全链路 | 在一个终端执行 `kubectl port-forward -n shortener svc/web 8080:80`，在另一个终端执行第 8.1 节的三条 `curl` 命令 | 创建返回 `201`，跳转返回 `302`，统计返回 `200` |

---

## 8. 阶段 5 的数据持久化验证

本阶段的目标是回答一个问题：删除数据库 Pod 之后，此前创建的短链接是否仍然能够跳转。

### 8.1 观察步骤

| 序号 | 动作 | 命令 | 预期结果 |
|---|---|---|---|
| 1 | 建立一条短链接 | `curl -s -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -d '{"url":"https://example.com/persist/check"}'` | 返回 `201` 与四个字段，记下 `code` 的取值 |
| 2 | 确认数据已经进入数据库 | `kubectl exec -n shortener postgres-0 -- psql -U shortener -d shortener -c 'select code, clicks from links'` | 表中出现上一步创建的短码 |
| 3 | 删除数据库 Pod | `kubectl delete po postgres-0 -n shortener` | 输出 `pod "postgres-0" deleted` |
| 4 | 观察重建过程 | `kubectl get po -n shortener -w` | 出现一个新的 `postgres-0`（名称不变），经过若干秒之后 `READY` 列变成 `1/1`；观察到这一行之后按 `Ctrl+C` 结束监视 |
| 5 | 重新查询数据库 | `kubectl exec -n shortener postgres-0 -- psql -U shortener -d shortener -c 'select code, clicks from links'` | 上一步创建的短码仍然存在，`clicks` 与删除之前一致 |
| 6 | 验证跳转 | `curl -i http://localhost:8080/<第 1 步的 code>` | 返回 `302` 与正确的 `Location` 字段 |

### 8.2 需要写进笔记的机制

第一条机制：`volumeClaimTemplates` 为每一个序号生成一个名称固定的 PVC，形如 `data-postgres-0`，这个 PVC 的生命周期与 StatefulSet 绑定而不与 Pod 绑定。Pod 被删除之后，控制器重建的新 Pod 名称仍然是 `postgres-0`，它挂载的仍然是同一个 PVC，因此数据文件的位置没有变化。

第二条机制：PVC 绑定的是独立于 Pod 生命周期的存储卷，因此删除 Pod 这个动作只影响计算资源，不影响数据。反过来说，数据丢失只有两种路径：删除 PVC，或者挂载了不提供持久化的卷类型（例如 `emptyDir`，它的生命周期与 Pod 相同）。

第三条机制：`postgres-0` 的重建过程不需要重新执行 `initdb`，因为数据目录中已经存在初始化完成的数据库簇，`postgres` 镜像的入口脚本检测到这一点之后直接启动数据库进程，这也解释了为什么重建速度快于第一次启动。

如果要做反例实验，可以在实验结束后执行 `kubectl delete pvc data-postgres-0 -n shortener`，再删除并重建 StatefulSet，此时数据库会重新初始化，表中的数据全部丢失。执行这条命令之前要清楚它的后果，因为本地没有其他备份。

---

## 9. 阶段 6：故障排查与事故演练

阶段 6 已从部署参考中拆出，作为独立实操文档维护：
[阶段 6：故障排查与事故演练手册](./TROUBLESHOOTING-LAB.md)。

手册按照事件响应流程组织：实验准备 → 影响评估 → 分层诊断 → 单一故障注入 → 证据分析 → 服务恢复与验收 → 技术复盘及限时综合诊断。内容包括七项可逆实验、CPU 节流可选练习、限时综合诊断、逐步操作命令和实验记录模板。首次执行应依照实验顺序完成，并通过每项实验的基线检查与恢复验收。

## 10. 常见错误对照表

| 现象 | 原因 | 定位方式 |
|---|---|---|
| Pod 状态是 `ErrImageNeverPull` 或者 `ImagePullBackOff` | 镜像没有加载到节点，或者 `imagePullPolicy` 是 `Always` | 执行 `kind load docker-image <镜像> --name shortener`；执行 `kubectl describe po` 查看 `Events` 段落中的拉取失败信息 |
| Pod 状态是 `CreateContainerConfigError` | `env` 引用的 ConfigMap 键或者 Secret 键不存在 | 执行 `kubectl describe po`，`Events` 段落中会写出缺失的键名；再执行 `kubectl get cm api-config -n shortener -o yaml` 核对实际存在的键 |
| 后端容器反复重启，日志中出现「连接数据库失败」并且错误文本包含 `connection refused` | Secret 中的 `DATABASE_URL` 主机名仍然是 `localhost` | 执行 `kubectl get secret postgres-secret -n shortener -o jsonpath='{.data.DATABASE_URL}' \| base64 -d` 查看实际取值，主机名必须是 `postgres` |
| `api` 的 Pod 长时间处于 `Pending` | 容器的 `requests` 之和超过了节点可分配的容量 | 执行 `kubectl describe po -l app=api -n shortener`，`Events` 段落中会出现 `Insufficient cpu` 或者 `Insufficient memory`；kind 的单节点集群容量有限，`requests` 要写得小一些 |
| `postgres-0` 处于 `Pending` 并且 PVC 处于 `Pending` | 集群中没有默认 StorageClass，PVC 无法动态供给 | 执行 `kubectl get storageclass` 确认存在标记为默认的条目；kind 创建的集群自带名为 `standard` 的默认 StorageClass |
| `web` 的容器反复重启，日志中出现 `host not found in upstream "api"` | `api` 这个 Service 不存在，或者它与 `web` 不在同一个命名空间 | 执行 `kubectl get svc -n shortener` 确认 Service 名称；nginx 在启动阶段解析上游主机名，解析不到时直接退出，因此这条错误的形态是重启而不是运行期报错 |
| 浏览器能够打开前端页面，但接口请求返回 `502` | `api` 的 Endpoints 是空的，也就是没有就绪的后端副本 | 执行 `kubectl get endpoints api -n shortener`；再执行 `kubectl get po -l app=api -n shortener` 确认 `READY` 列，最后用 `kubectl describe po` 与后端日志定位就绪检查失败的原因 |
| `READY` 列是 `0/1` 而 `RESTARTS` 列是 `0` | 就绪探针失败，进程本身正常 | 这是就绪探针失败的标准形态，先执行 `kubectl describe po` 查看探针失败的事件，再执行 `kubectl exec` 直接请求检查端点，观察响应体中的具体取值 |
| 修改 ConfigMap 之后 Pod 的行为没有变化 | 用 `env` 方式注入的取值在容器创建时固化成环境变量 | 执行 `kubectl rollout restart deploy/<名称> -n shortener` 触发滚动重建；执行 `kubectl exec` 后在容器内用 `env` 命令核对实际生效的取值 |
| Windows 浏览器无法访问 `http://localhost:30080` | kind 的节点是容器，NodePort 没有映射到宿主机 | 改用 `kubectl port-forward -n shortener svc/web 8080:80`，或者在创建集群时配置 `extraPortMappings` |
| 删除 `postgres-0` 之后数据丢失 | 误删了 PVC，或者挂载的卷类型不提供持久化 | 执行 `kubectl get pvc -n shortener` 确认 `data-postgres-0` 是否存在并且状态是 `Bound`；检查 StatefulSet 的 `volumeClaimTemplates` 是否被误写成 `emptyDir` |
| `kubectl exec` 报容器中没有可用的 shell | 镜像中不含 shell 程序 | 本项目的两个业务镜像都基于 alpine，包含 `sh`；如果换成不含 shell 的镜像，只能依靠日志与探针输出定位问题 |

---

## 11. 需要写进笔记的机制

| 机制 | 结论 |
|---|---|
| StatefulSet 与 Deployment 的差异 | 只有当工作负载需要稳定网络标识或者需要每个副本独占存储时，才使用 StatefulSet；无状态服务使用 Deployment，因为它的副本可以互换，名称不需要保持稳定 |
| `volumeClaimTemplates` 与 Pod 生命周期的关系 | PVC 的生命周期与 StatefulSet 绑定而不与 Pod 绑定，因此删除 Pod 不会丢数据，删除 PVC 才会丢数据 |
| 就绪探针与存活探针的分工 | 判断「是否可以把流量交给这个 Pod」时使用就绪探针并允许它检查外部依赖，判断「是否必须重启这个进程」时使用存活探针并且不要让它检查外部依赖 |
| ConfigMap 与 Secret 的更新时机 | 以环境变量方式注入的取值在容器创建时固化，修改之后必须重启工作负载；以卷的方式挂载的取值会自动更新 |
| Service 与 Endpoints 的关系 | Service 只提供稳定的虚拟地址与名称，真正决定流量去向的是 Endpoints；Endpoints 的内容由选择器与就绪状态共同决定 |
| NodePort 在 kind 中的可达范围 | NodePort 绑定在节点容器上，因此只有在创建集群时把该端口映射到宿主机，Windows 侧的浏览器才能直接访问 |
| 宽限期与优雅退出的关系 | `terminationGracePeriodSeconds` 必须大于应用完成收尾工作所需的时限，否则收尾过程被 `SIGKILL` 中断并且产生数据不一致 |
| 资源限制与容器退出的关系 | `limits.memory` 被超过时，内核以 `OOMKilled` 终止容器，退出码是 `137`，这个过程不会在应用日志中留下记录 |

---

## 12. 你的任务清单

| 序号 | 你要做的动作 | 产物 | 完成标志 |
|---|---|---|---|
| 0 | 完成第 0.2 节的四项前置检查，安装 kubectl，创建 kind 集群，构建并加载两个镜像 | 可用的集群与两个已加载的镜像 | `kubectl get nodes` 输出一个 `Ready` 状态的节点；`docker exec shortener-control-plane crictl images` 中能看到两个业务镜像 |
| 1 | 编写 `00-namespace.yaml` | 命名空间清单 | `kubectl get ns shortener` 的状态是 `Active` |
| 2 | 编写 `10-config.yaml` | ConfigMap 与 Secret 各一个 | 两个对象的键与第 2.1 节的表格逐项对应 |
| 3 | 编写 `20-postgres.yaml` | Headless Service 与 StatefulSet | `postgres-0` 的 `READY` 是 `1/1`，PVC 是 `Bound` |
| 4 | 编写 `30-redis.yaml` | ClusterIP Service 与 Deployment | `redis` 的 Pod 的 `READY` 是 `1/1` |
| 5 | 编写 `40-api.yaml` | ClusterIP Service 与两个副本的 Deployment | 两个 `api` Pod 的 `READY` 是 `1/1`，就绪检查端点返回 `ready` |
| 6 | 编写 `50-web.yaml` | NodePort Service 与两个副本的 Deployment | 两个 `web` Pod 的 `READY` 是 `1/1`，`kubectl port-forward` 之后浏览器可以创建与跳转 |
| 7 | 执行第 7 节与第 8 节的验收，并且记录实测取值 | 现象记录 | 九条验收命令与六条持久化验证步骤全部符合预期 |
| 8 | 按 [阶段 6 故障排查与事故演练手册](./TROUBLESHOOTING-LAB.md) 完成七项可逆实验、一次限时综合诊断，并填写实验记录 | 事故演练记录 | 能依据实测证据定位根因、恢复服务并验证业务；综合诊断至少 8/10 分 |
| 9 | 把第 11 节的八条机制整理成笔记，并且按第 3 周的笔记结构补上「全流程工作链路」与「十分钟速记卡」 | 当天笔记 | 笔记中包含从 `kubectl apply` 到 Pod 就绪的全部阶段，每个阶段都写明执行者、输入、动作与观察方式 |
