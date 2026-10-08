# 阶段 6：故障排查与事故演练手册

> 本手册是 [Kubernetes 部署指引](./README.md) 阶段 6 的配套实训文档。面向尚未独立完成 Kubernetes 故障演练的学习者，提供标准化的实验顺序、诊断命令、预期证据、恢复步骤与验收标准。

## 1. 整体框架

本手册按照生产环境常见的事件响应流程组织，而非按命令类型组织：

1. **实验准备**：确认目标集群、工作负载状态、业务基线与恢复路径。
2. **影响评估**：从客户端入口开始，确定受影响的页面、API、短链跳转或点击统计功能。
3. **分层诊断**：客户端与入口 → Service 与 EndpointSlice → Pod 与控制器 → 应用进程 → 依赖、DNS 与网络 → 数据层。
4. **故障注入**：每次仅改变一个配置项或资源对象，并记录注入前的基线。
5. **证据分析**：基于观测结果提出假设，通过针对性检查验证或排除假设，最终确定根因。
6. **服务恢复与验收**：恢复原始配置，确认工作负载稳定，并验证入口、服务端点、健康接口和业务请求。
7. **复盘与能力评估**：记录机制、预防措施和适用边界，最后完成限时综合诊断。

### 建议学习顺序

首次执行时，建议按以下顺序完成：

1. 完成第 3 节的环境检查，并保持 `kubectl port-forward` 会话运行。
2. 阅读第 4 节的诊断流程，理解各层检查的目的与证据边界。
3. 依次完成第 5 至第 11 节的实验。每项实验均须先恢复，再开始下一项。
4. 使用第 13 节的记录模板保存实际观测结果；预期现象不能替代实测证据。
5. 最后完成第 12 节的限时综合诊断；建议由另一位参与者注入故障并提供症状。

**安全条件：**仅允许在个人 kind 学习集群执行故障注入。若当前上下文不是 `kind-shortener`、实验基线未通过、命令执行失败、观测结果与预期不符，或无法确认命令的影响范围，应停止实验并完成诊断；不得在共享集群或生产环境执行本手册中的注入操作。

---

## 2. 演练背景与目标

演练场景为短链接服务完成版本发布后，出现页面可访问但 API 请求失败、短链无法跳转或点击统计延迟等问题。参与者作为事件响应人员，需要评估影响范围，按系统层次收集证据，确定故障根因，执行可回滚的恢复操作，并验证服务及业务功能恢复。

目标是在限时诊断中完成以下工作：描述影响范围、定位故障层、依据证据确认根因、恢复服务、验证业务功能，并提出针对性预防措施。Pod 的 `Running` 状态仅表示容器正在运行，不应单独作为服务可用性的判据。

## 3. 实验规则与环境准备

1. 仅在个人 kind 学习集群中执行实验，不得在共享或生产集群中注入故障。
2. 每次仅执行一项实验；开始下一项前，必须完成当前实验的恢复验收。
3. 不删除 PVC 或命名空间，不清理 Redis 数据，不修改数据库 Secret 中的凭据。本手册中的注入操作应可逆。
4. 注入故障前记录基线。重启次数应比较注入前后的差值，不应要求绝对值为零。
5. 每项实验至少保存一条关键证据，例如 Kubernetes Events、容器日志、EndpointSlice、HTTP 状态码或数据库查询结果，并保存恢复后的验证结果。
6. 命令失败或观测结果与预期不符时，应停止后续注入，先确定偏差原因。

命令中的尖括号文本（例如 `<api-pod名称>` 和 `<code>`）表示待替换占位符。执行命令前，必须替换为实际资源名称或短码；不得原样执行占位符。

首先检查当前 Kubernetes context 与集群节点。当前 context 应为 `kind-shortener`；如不是，只有在确认该 context 指向个人学习集群后，才执行 `kubectl config use-context kind-shortener`。

```bash
kubectl config current-context
kubectl get nodes
kubectl get deploy,sts,pods,svc,pvc -n shortener -o wide
kubectl rollout status deploy/api -n shortener --timeout=90s
kubectl rollout status deploy/web -n shortener --timeout=90s
```

启动本地端口转发，并在实验期间保持该进程运行。后续业务检查均通过本地 8080 端口进行：

```bash
kubectl port-forward -n shortener svc/web 8080:80
```

- port：Service 对集群内提供的端口。
- targetPort：流量最终转发到 Pod 的端口。
- nodePort：当 Service 类型为 NodePort 时，节点上开放的端口；不是自动映射到宿主机的端口。

在另一个终端采集基线，并将实际输出保存在实验记录中。不得以预期输出替代实际观测；无需将真实短码、终端截图或集群内部地址提交至仓库。

```bash
curl -i http://localhost:8080/
curl -i http://localhost:8080/api/healthz
curl -i http://localhost:8080/api/readyz
kubectl get pods -n shortener -o wide
kubectl get endpointslice -n shortener -l kubernetes.io/service-name=api
kubectl get events -n shortener --sort-by=.lastTimestamp
```

基线通过条件：两个 API 副本均为 Ready；`healthz` 与 `readyz` 均返回 HTTP 200；`readyz` 中 PostgreSQL 和 Redis 的检查结果均为 `ok`；API Service 的 EndpointSlice 包含就绪后端。若集群已有业务数据，应记录数据量或选择专用测试短码；不得通过清理现有数据构造实验环境。

## 4. 标准诊断流程

按以下层次逐步诊断。每项检查均应对应一个待验证的问题；发现异常后，应沿资源引用关系和事件信息继续验证其直接影响及根因，不应将此顺序视为相互独立的线性流程。

| 顺序 | 系统层次 | 诊断问题 | 证据来源 |
|---|---|---|---|
| 1 | 客户端与入口 | 哪些用户功能受影响？具体 HTTP 状态码、响应时间和错误内容是什么？ | `curl -i`、浏览器开发者工具 Network 面板 |
| 2 | Service 与端点 | Service 是否存在？EndpointSlice 是否包含 Ready 后端？ | `kubectl get svc,endpointslice -n shortener` |
| 3 | 工作负载与 Pod | 控制器期望副本数是否满足？Pod 是否已调度、启动并通过探针？ | `kubectl get pods -o wide`、`kubectl describe pod`、`kubectl rollout status` |
| 4 | 应用进程 | 容器是否退出？应用日志是否包含启动或请求错误？ | `kubectl logs`、`kubectl logs --previous` |
| 5 | 依赖与网络 | DNS 是否解析？目标端口是否可连接？依赖是否就绪？ | `nslookup`、`wget`、`/api/readyz`、依赖 Pod 与 Service 状态 |
| 6 | 数据与业务恢复 | 恢复后业务链路和持久化数据是否正确？ | 创建、跳转、列表 API；PostgreSQL 查询；PVC 状态 |

诊断时应遵守以下判断边界：

- `Running` 表示容器当前处于运行状态，不等价于 Pod Ready、Service 后端可用或业务请求成功。
- Pod 的 `RESTARTS` 是累计值。应比较实验前后的变化，并结合 `Last State`、Events 与 `kubectl logs --previous` 判断本次故障。

## 5. 实验一：镜像拉取失败（镜像与发布控制器）

** 场景：**发布后新 API 副本未能进入 Ready 状态。诊断目标是区分镜像拉取失败与应用进程启动失败。

将 API Deployment 的镜像标签修改为预期不可用的标签：

```bash
kubectl set image deploy/api api=shortener-api:notexist -n shortener
kubectl rollout status deploy/api -n shortener --timeout=60s
```

第二条命令在新副本无法就绪时可能超时并返回非零状态；这是本实验的预期结果。出现超时后继续执行下方的证据采集命令。

采集以下证据：

```bash
kubectl get pods -n shortener -l app=api -o wide
kubectl describe pods -n shortener -l app=api
kubectl get events -n shortener --sort-by=.lastTimestamp
kubectl get endpointslice -n shortener -l kubernetes.io/service-name=api
```

预期新 ReplicaSet 的 Pod 进入 `ErrImagePull` 或 `ImagePullBackOff`，Events 中显示镜像拉取失败。当前清单使用 `imagePullPolicy: IfNotPresent`；若本地镜像不存在，kubelet 会尝试从镜像仓库拉取，拉取失败后通常进入 `ImagePullBackOff`。`ErrImageNeverPull` 对应 `imagePullPolicy: Never` 且本地镜像不存在的情形，不是当前清单的预期状态。滚动更新期间，Deployment 通常会保留旧的 Ready 副本，因此旧版本可能继续提供服务。由于容器进程尚未启动，`RESTARTS` 不一定增加。

诊断说明应回答：为什么该现象不属于应用进程崩溃？如何根据 Events、镜像名称、`imagePullPolicy` 和 kind 节点镜像清单，区分镜像不存在与本地镜像未加载？

恢复与验收：

```bash
kubectl rollout undo deploy/api -n shortener
kubectl rollout status deploy/api -n shortener --timeout=90s
curl -i http://localhost:8080/api/readyz
```

`rollout undo` 仅用于本项由 `kubectl set image` 创建、且历史版本符合预期的演练。其他实验应显式恢复被修改的字段，避免回滚整个 Deployment 引入非预期变更。

## 6. 实验二：就绪探针失败（readiness probe）

**场景：**API 进程可以响应，但就绪探针路径配置错误。诊断目标是验证 readiness probe 对流量准入的影响，并与 liveness probe 的重启行为区分。

```bash
kubectl patch deploy/api -n shortener --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/path","value":"/api/not-exist"}]'
kubectl rollout status deploy/api -n shortener --timeout=60s
kubectl get pods -n shortener -l app=api -o wide
kubectl describe pods -n shortener -l app=api
kubectl get endpointslice -n shortener -l kubernetes.io/service-name=api
```

由于新副本无法通过 readiness probe，`kubectl rollout status` 可能等待至超时并返回非零状态；该结果符合本实验预期。超时后继续收集 Pod、Events 与 EndpointSlice 证据。

预期新副本为 `Running`、`READY 0/1`，Events 中出现 HTTP 404 的 Readiness probe failure，新副本不进入 Service 的可用后端集合。滚动更新策略会保留旧的 Ready 副本，因此 EndpointSlice 仍可能包含旧副本地址；应按 Pod 地址核对新旧副本，而不能要求 EndpointSlice 必须为空。readiness probe 失败本身不会触发容器重启。

恢复：

```bash
kubectl patch deploy/api -n shortener --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/path","value":"/api/readyz"}]'
kubectl rollout status deploy/api -n shortener --timeout=90s
curl -i http://localhost:8080/api/readyz
```

**机制要点：**readiness probe 失败会使 Pod 不具备接收 Service 流量的资格；liveness probe 失败达到阈值后，kubelet 会重启容器。两类探针的检测目标和控制行为不同。

## 7. 实验三：Service selector 与 Pod 标签不匹配

**场景：**前端页面可访问，但经 nginx 代理的 API 请求返回 502；API Pod 本身保持 Ready。诊断目标是确认 Service selector 与 Pod 标签之间的映射关系。

```bash
kubectl patch svc api -n shortener --type=merge \
  -p='{"spec":{"selector":{"app":"api-lab-mismatch"}}}'
kubectl get pods -n shortener -l app=api
kubectl get svc api -n shortener -o yaml
kubectl get endpointslice -n shortener -l kubernetes.io/service-name=api -o yaml
curl -i http://localhost:8080/api/healthz
```

预期 API Pods 仍为 Ready，但 API Service 对应的 EndpointSlice 不包含可用地址；经 web/nginx 代理的 API 请求返回 502。根因是 Service selector 与 Pod 标签不匹配，而不是 API 进程或镜像异常。

恢复：

```bash
kubectl patch svc api -n shortener --type=merge \
  -p='{"spec":{"selector":{"app":"api"}}}'
kubectl get endpointslice -n shortener -l kubernetes.io/service-name=api
curl -i http://localhost:8080/api/healthz
curl -i http://localhost:8080/api/readyz
```

**机制要点：**Service 提供稳定的虚拟入口，EndpointSlice 表示该 Service 当前发现的后端端点。只有标签满足 selector 条件的 Pod 才可能成为后端；Pod Ready 与 Service 后端可用是不同层次的状态。

## 8. 实验四：ConfigMap 配置错误导致的依赖连接故障

**场景：**API 新副本启动失败，但旧 ReplicaSet 可能仍在提供服务。本实验分别注入 DNS 名称错误和目标端口错误，比较两类网络故障的证据。

首先注入 DNS 名称错误。由于 ConfigMap 通过环境变量注入，已有容器不会自动读取更新后的值，因此必须滚动重启 Deployment：

```bash
kubectl patch cm api-config -n shortener --type=merge \
  -p='{"data":{"REDIS_ADDR":"redis-wrong:6379"}}'
kubectl rollout restart deploy/api -n shortener
kubectl get pods -n shortener -l app=api -o wide
kubectl get events -n shortener --sort-by=.lastTimestamp
kubectl logs -n shortener -l app=api --all-containers=true --prefix=true --tail=40
```

应用启动时会 Ping Redis。错误地址通常导致新容器启动失败并由 kubelet 重启；旧 ReplicaSet 的 Ready 副本可能仍在提供服务。检查失败副本的当前日志及前一实例日志，确认错误信息包含 Redis 地址解析失败：

```bash
kubectl get pods -n shortener -l app=api -o wide
kubectl logs -n shortener '<故障-api-pod名称>' --tail=40
kubectl logs -n shortener '<故障-api-pod名称>' --previous --tail=40
```

恢复到正确地址并等待 rollout 完成：

```bash
kubectl patch cm api-config -n shortener --type=merge \
  -p='{"data":{"REDIS_ADDR":"redis:6379"}}'
kubectl rollout restart deploy/api -n shortener
kubectl rollout status deploy/api -n shortener --timeout=90s
curl -i http://localhost:8080/api/readyz
```

再做端口错误这一轮，确认 DNS 与 TCP 连接失败不是同一类证据：

```bash
kubectl patch cm api-config -n shortener --type=merge \
  -p='{"data":{"REDIS_ADDR":"redis:6380"}}'
kubectl rollout restart deploy/api -n shortener
kubectl get events -n shortener --sort-by=.lastTimestamp
kubectl logs -n shortener -l app=api --all-containers=true --prefix=true --tail=40
```

此时 DNS 名称 `redis` 应能够解析，但 Redis Service 未在 6380 端口监听。比较两轮实验的实际日志：名称解析失败通常包含 `no such host`；目标端口不可连接时，可能出现 `connection refused` 或连接超时，具体信息取决于网络实现。随后将地址恢复为 `redis:6379` 并等待 rollout 成功。记录实际证据，不应将预期文本替代观测结果。

```bash
kubectl patch cm api-config -n shortener --type=merge \
  -p='{"data":{"REDIS_ADDR":"redis:6379"}}'
kubectl rollout restart deploy/api -n shortener
kubectl rollout status deploy/api -n shortener --timeout=90s
curl -i http://localhost:8080/api/readyz
```

**机制要点：**ConfigMap 对象更新不代表进程已读取新值；以环境变量注入的配置在容器创建时确定。`kubectl get cm` 只能证明控制面配置已更新，Pod 的实际环境与启动日志才反映运行配置。

## 9. 实验五：内存 OOM 与 CPU 节流的诊断对比

**场景：**API 副本发生重启，或请求延迟上升。诊断目标是区分内存超限导致的容器终止与 CPU 配额导致的节流。

### A. 内存超限

首先记录 Deployment 的资源配置和各 Pod 当前的重启次数：

```bash
kubectl get deploy api -n shortener -o yaml
kubectl get pods -n shortener -l app=api -o wide
```

同时调整 memory request 与 limit，确保 request 不大于 limit。`16Mi` 是用于触发内存压力的实验值，容器可能很快发生 OOM：

```bash
kubectl set resources deploy/api -n shortener \
  --requests=cpu=50m,memory=16Mi \
  --limits=cpu=500m,memory=16Mi
kubectl get pods -n shortener -l app=api -w
```

在另一个终端取证：

```bash
kubectl describe pods -n shortener -l app=api
kubectl get pods -n shortener -l app=api
kubectl get events -n shortener --sort-by=.lastTimestamp
kubectl logs -n shortener '<发生重启的-api-pod名称>' --previous --tail=40
```

若观测到 `OOMKilled`、退出码 137 且重启次数增加，说明容器进程因超过内存限制被内核终止；应用可能未能在终止前写入错误日志。若状态为 `Pending` 或 Deployment 更新被拒绝，应先检查 Events，以区分调度约束或资源配置校验问题。滚动更新期间旧副本可能继续提供服务。

恢复到清单中的基线：

```bash
kubectl set resources deploy/api -n shortener \
  --requests=cpu=50m,memory=64Mi \
  --limits=cpu=500m,memory=256Mi
kubectl rollout status deploy/api -n shortener --timeout=90s
```

### B. CPU 节流（可选，需要产生请求负载）

CPU limit 会限制容器可使用的 CPU 配额，通常导致节流而不是像内存 limit 一样直接终止进程。没有请求负载时，仅降低 CPU limit 不足以验证节流现象；单次请求延迟也不足以作为性能结论。

将 API Deployment 的 CPU limit 临时调整为 `50m`，保持 request 为 `50m`，然后对 `/api/links?limit=100` 产生并发请求，并与基线配置下的多轮结果比较。该操作作用于整个 Deployment，并触发滚动更新。若本机没有 `hey` 或其他 HTTP 负载生成工具，可将本小节记录为“环境限制，未执行”，无需为完成阶段安装工具。若集群已部署 Metrics Server，可使用 `kubectl top pod -n shortener` 观察 CPU 使用量；未部署时应记录该指标不可用。

```bash
kubectl set resources deploy/api -n shortener \
  --requests=cpu=50m,memory=64Mi \
  --limits=cpu=50m,memory=256Mi
kubectl rollout status deploy/api -n shortener --timeout=90s
hey -n 300 -c 10 'http://localhost:8080/api/links?limit=100'
```

验证重点：Pod 通常仍保持 Ready，CPU 限制本身不应导致重启；负载下应比较多轮请求的吞吐量与延迟分布。完成后将 API 资源恢复至基线并等待 rollout 完成。本实验用于说明 CPU 配额机制，不构成容量评估或性能基准。

```bash
kubectl set resources deploy/api -n shortener \
  --requests=cpu=50m,memory=64Mi \
  --limits=cpu=500m,memory=256Mi
kubectl rollout status deploy/api -n shortener --timeout=90s
```

**机制要点：**OOM 是内存上限触发的容器终止；CPU limit 通常造成 CPU 节流。两者的状态表现、诊断证据和缓解措施不同。

## 10. 实验六：liveness probe 对依赖故障的错误耦合

**场景：**Redis Service 暂时没有可用后端。通过将 liveness probe 错误配置为检查 `readyz`，观察依赖故障对 API 容器重启行为的影响。该实验可能导致 API 暂时不可用，执行前必须确认此前的实验均已恢复。

先将 API 的 liveness probe 路径修改为会检查 Redis 的 `readyz`，再修改 Redis Service selector，使其不再匹配 Redis Pod。Redis Deployment 与 Pod 保持运行，不会因本实验删除 Redis 数据：

```bash
kubectl patch deploy/api -n shortener --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/livenessProbe/httpGet/path","value":"/api/readyz"}]'
kubectl rollout status deploy/api -n shortener --timeout=90s
kubectl patch svc redis -n shortener --type=merge \
  -p='{"spec":{"selector":{"app":"redis-lab-mismatch"}}}'
kubectl get endpointslice -n shortener -l kubernetes.io/service-name=redis
kubectl get pods -n shortener -l app=api
kubectl describe pods -n shortener -l app=api
```

预期 Redis Service 的 EndpointSlice 不包含可用端点，API 的 `readyz` 返回非 200。由于 liveness probe 被配置为调用该端点，API 容器将在探针连续失败达到阈值后重启。记录重启次数的变化及 `Liveness probe failed` Events。实际重启时间由探针周期、超时和失败阈值共同决定。

按以下顺序恢复：首先恢复 Redis Service selector，使依赖端点重新可用；随后将 liveness probe 恢复为 `/api/healthz`：

```bash
kubectl patch svc redis -n shortener --type=merge \
  -p='{"spec":{"selector":{"app":"redis"}}}'
kubectl get endpointslice -n shortener -l kubernetes.io/service-name=redis
kubectl patch deploy/api -n shortener --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/livenessProbe/httpGet/path","value":"/api/healthz"}]'
kubectl rollout status deploy/api -n shortener --timeout=90s
curl -i http://localhost:8080/api/readyz
```

**机制要点：**重启 API 进程不能修复 Redis 故障。liveness probe 用于判断容器是否需要重启，依赖可用性应由 readiness probe 反映。将依赖健康检查耦合到 liveness 可能把局部依赖故障放大为容器重启风暴。

## 11. 实验七：Pod 终止时的点击增量写回（SIGTERM）

**场景：**Deployment 更新或节点维护会终止 API Pod。本实验验证 Kubernetes 发送 SIGTERM 后，应用能否在终止宽限期内完成 HTTP 服务收尾及最后一次点击增量写回。

本实验仅创建并访问一条专用短链，不删除数据库或 Redis 数据。开始前确认前置检查通过。首先将点击写回周期临时设置为 `5m`，为创建测试数据、检查 Redis 增量和触发 Pod 终止预留观察时间，并等待 API Deployment 完成滚动更新：

```bash
kubectl patch cm api-config -n shortener --type=merge \
  -p='{"data":{"CLICK_FLUSH_INTERVAL":"5m"}}'
kubectl rollout restart deploy/api -n shortener
kubectl rollout status deploy/api -n shortener --timeout=90s
```

通过端口转发创建测试短链并记录响应中的 `code`。将后续命令中的 `<code>` 替换为实际短码。创建请求应返回 HTTP 201；访问短链应返回 HTTP 302。请求中不得添加 `curl -L`，以避免跟随重定向访问外部目标站点。

```bash
curl -i -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/k8s-shutdown-test"}'
curl -i 'http://localhost:8080/<code>'
```

查询 PostgreSQL 中该短码的初始点击数，并检查 Redis 中对应的点击增量键：

```bash
kubectl exec -n shortener postgres-0 -- \
  psql -U shortener -d shortener -c "select code, clicks from links where code = '<code>'"
kubectl exec -n shortener deploy/redis -- redis-cli GET 'clicks:<code>'
```

此时数据库中的测试记录应显示 `clicks = 0`，Redis 点击增量键应包含本次访问产生的增量。应在 5 分钟周期到期前完成 Pod 终止操作。若 Redis 键不存在或数据库计数已变化，应检查是否已触发周期性写回；如观察窗口已失效，应创建另一条测试短链并重复验证。

在终端 A 中启动目标 API Pod 的日志跟踪，并保持该命令运行：

```bash
kubectl get pods -n shortener -l app=api
kubectl logs -f -n shortener 'pod/<api-pod名称>'
```

在终端 B 中删除同一个 API Pod。Deployment 控制器将创建替代 Pod。当前 Pod 模板设置的终止宽限期为 30 秒；收到删除请求后，kubelet 会请求容器运行时向容器主进程发送 SIGTERM，并在宽限期结束后仍未退出时发送 SIGKILL：

```bash
kubectl delete pod '<api-pod名称>' -n shortener --wait=false
kubectl get pods -n shortener -l app=api -w
```

在终端 A 中观察原 Pod 的终止日志，重点确认是否出现收到停止信号、执行最终写回及服务停止等日志。然后在终端 B 中确认替代 Pod Ready，并查询数据库：

```bash
kubectl exec -n shortener postgres-0 -- \
  psql -U shortener -d shortener -c "select code, clicks from links where code = '<code>'"
kubectl get pods -n shortener -l app=api
```

应用收到 SIGTERM 后会停止接收新请求、关闭 HTTP 服务，并尝试执行最后一次点击写回。当前 Deployment 的 `terminationGracePeriodSeconds: 30` 大于配置中的 `SHUTDOWN_TIMEOUT: 10s`，为应用收尾提供时间窗口。两个 API 副本共享 Redis，因此终止任一副本触发的最终写回可能处理共享 Redis 中已收集的增量；实验结论应以数据库结果和终止日志为依据。

无论观测结果如何，均须恢复原始的 5 秒写回周期，并等待 API Deployment 完成滚动更新：

```bash
kubectl patch cm api-config -n shortener --type=merge \
  -p='{"data":{"CLICK_FLUSH_INTERVAL":"5s"}}'
kubectl rollout restart deploy/api -n shortener
kubectl rollout status deploy/api -n shortener --timeout=90s
curl -i http://localhost:8080/api/readyz
```

若点击数未增加，不得通过删除数据库或清空 Redis 重试。应检查 Redis 点击增量键、Pod 终止日志、最终写回错误及 5 分钟周期写回是否已经发生。本实验仅验证应用的 SIGTERM 收尾路径，不代表在 SIGKILL、节点断电或其他非优雅终止场景下能够保证零数据丢失。

## 12. 限时综合诊断：前端可访问但 API 服务异常

本练习用于评估独立诊断能力。由一位参与者担任故障注入者，另一位担任诊断者。注入者从下表选择一项故障，只向诊断者提供症状，不提供注入命令。诊断者限时十分钟，先收集证据并记录假设，再实施修复。

故障注入者每轮仅选择一项，并仅向诊断者提供“注入者提供的症状”列中的描述：

| 编号 | 注入者提供的症状 | 注入命令 | 故障层 |
|---|---|---|---|
| A | 首页可访问，所有 API 请求返回 502 | `kubectl patch svc api -n shortener --type=merge -p='{"spec":{"selector":{"app":"api-lab-mismatch"}}}'` | Service / EndpointSlice |
| B | API 仍可访问，但新版本滚动更新无法完成 | `kubectl patch deploy/api -n shortener --type=json -p='[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/path","value":"/api/not-exist"}]'` | 新副本 readiness / 滚动发布 |
| C | API 仍有可用副本，但新建副本启动失败并反复重启 | `kubectl patch cm api-config -n shortener --type=merge -p='{"data":{"REDIS_ADDR":"redis-wrong:6379"}}' && kubectl rollout restart deploy/api -n shortener` | 配置 / 依赖启动 |

诊断者应按第 4 节的标准流程操作，并在修改集群前记录初始假设及其证据。未经根因确认，不应执行 `rollout restart` 或 `rollout undo`。确定根因后，使用对应实验中的明确恢复值，等待 rollout 完成，并验证首页、`healthz`、`readyz` 及至少一个业务请求。

评分（每项 0～2 分，总分 10）：

| 能力 | 0 分 | 1 分 | 2 分 |
|---|---|---|---|
| 影响评估 | 未能描述用户可见症状 | 区分页面与 API 影响 | 明确受影响路径、副本范围及用户影响 |
| 证据采集 | 未采集证据或直接修改配置 | 执行检查但不能说明目的 | 每项证据均对应待验证的假设 |
| 根因判定 | 仅提出猜测 | 定位到异常对象 | 使用证据排除相邻故障层并确认根因 |
| 恢复验收 | 仅确认 Pod 处于 Running | 确认 Pod Ready | 验证端点、健康接口及业务请求 |
| 技术复盘 | 仅复述操作步骤 | 能说明相关 Kubernetes 机制 | 能说明预防措施、风险及结论适用边界 |

总分达到 8 分及以上，且未发生未经证据支持的重启或资源删除操作，视为通过。后续轮次应互换故障注入者与诊断者角色。

## 13. 实验记录模板与阶段验收

每项实验均应使用以下模板记录实际观测结果。预期现象应与实测结果分开记录：

```text
实验名称：
用户可见症状与影响范围：
实验基线（Pod Ready 状态、重启次数、EndpointSlice、HTTP 状态）：
故障注入方式与时间：
初始假设及其依据：
诊断命令与实际输出：
根因结论：
恢复操作：
恢复验收（Pod、EndpointSlice、健康接口、业务请求及相关数据）：
预防或监控措施：
面试问题与回答要点：
```

阶段 6 完成标准：

- 完成第 5 至第 11 节的七项故障实验。若缺少负载生成工具，可将 CPU 节流实验记录为“环境限制，未执行”，并说明原因。
- 完成一次限时综合诊断，得分不低于 8/10。
- 能解释 readiness probe、liveness probe、Service/EndpointSlice、ConfigMap 环境变量更新机制、OOM 与 CPU 节流、Deployment 滚动更新及 SIGTERM 收尾。
- 每项实验均包含真实观测证据、明确恢复操作及恢复验收结果。命令执行成功或 Pod 处于 Running 均不能单独作为完成依据。

---
