#!/usr/bin/env bash
# =============================================================================
# deploy-local.skeleton.sh —— 本地 kind 部署与验收脚本的骨架
# =============================================================================
#
# 用途：
#   把交付链路里最后的四个动作固定成一条可重复执行的命令，也就是
#   「取得指定提交标识的两个镜像 → 导入本机 kind 集群的节点 → 更新 API 与 Web
#     两个 Deployment 的容器镜像 → 等待 rollout 完成 → 通过端口转发检查健康端点」。
#   这条链路的前三段（提交 → 持续集成 → 镜像发布到 GHCR）已经由 ci.yml 与
#   publish-images.yml 完成，本脚本负责第四段到第七段。
#
# 用法：
#   ./deploy-local.sh <镜像标签>
#   其中 <镜像标签> 是发布工作流推送到 GHCR 的完整提交标识（40 位十六进制字符串）。
#
# 三条设计约束（它们决定了脚本的结构，改动结构时必须同时复核这三条约束）：
#   1. 本脚本只允许操作本机的个人 kind 集群。因此第一步必须核对当前 context；
#      当 context 与预期取值不一致时，脚本立即以非零退出码结束，
#      并且在退出之前不得执行任何修改类动作。
#   2. 任一阶段失败时脚本以非零退出码结束，不使用回退逻辑掩盖失败；
#      否则调用方会得到「脚本成功而部署失败」的结果。
#   3. 同一个镜像标签被反复部署时结果必须一致（幂等），
#      原因是同一个标签会被反复使用来对账当前集群的镜像版本。
#
# 需要我补全的地方有两类：
#   · 配置区中每一项的取值（下面用 ___ 表示）；
#   · 五个函数体内标注为 ___ 的命令。

set -euo pipefail

# pf_pid 保存后台端口转发进程的进程号。它必须声明为全局变量：
# EXIT trap 在脚本退出时执行，那时函数内部的局部变量已经不存在。
pf_pid=""

# =============================================================================
# 配置区：下面每一项的取值都由我填写
# =============================================================================
#
# 每一项都给出「要填什么」「判定条件」「写错的后果」三条注释；取值本身留空。
# 更完整的决策依据见 code/week3/day5/README.md 第三节。

CLUSTER_NAME="shortener"
# CLUSTER_NAME：本机个人 kind 集群的名称。
#   要填什么：集群创建时使用的名称。
#   判定条件：与 `kind create cluster --name <名称>` 中的取值逐字一致。
#   写错的后果：`kind load docker-image --name <名称>` 会报找不到该集群，
#               镜像不会被导入任何节点，Pod 随后停在 ErrImageNeverPull 或 ImagePullBackOff。

EXPECTED_CONTEXT="kind-shortener"
# EXPECTED_CONTEXT：允许本脚本操作的 context 名称。
#   要填什么：kind 为集群创建的 context 名称，形如 kind-<集群名称>。
#   判定条件：与 `kubectl config current-context` 的输出逐字一致。
#   写错的后果：把判定放宽成「包含 kind 字样」时，脚本会误操作本机的其它 kind 集群；
#               取值写得与实际不一致时，脚本会永远拒绝执行。

NAMESPACE="shortener"
# NAMESPACE：业务对象所在的命名空间。
#   要填什么：全部清单使用的命名空间名称。
#   判定条件：与 `00-namespace.yaml` 中的 metadata.name 一致。
#   写错的后果：取值不一致时，`kubectl set image` 与 `kubectl rollout status`
#               会在空命名空间中查找对象并报找不到资源。

REGISTRY="ghcr.io"
# REGISTRY：镜像库的域名。
#   要填什么：本项目使用的镜像库域名。
#   判定条件：与发布工作流中使用的镜像库域名一致。
#   写错的后果：取值不一致时，镜像名称拼接错误，docker pull 会报仓库不存在。

IMAGE_NAMESPACE="duramentee"
# IMAGE_NAMESPACE：镜像路径中的命名空间，也就是仓库所有者名称的小写形式。
#   要填什么：仓库所有者名称经过小写化之后的取值。
#   判定条件：镜像库要求镜像路径中的命名空间全部使用小写字母。
#   写错的后果：保留大写字母时，docker pull 会因为路径非法而失败。

API_IMAGE_NAME="shortener-api"
# API_IMAGE_NAME：后端镜像的包名。
#  要填什么：发布工作流中后端镜像使用的包名。
#  判定条件：与发布工作流里后端镜像名称中冒号之前的部分一致。
#  写错的后果：取值不一致时，脚本拉取的是不存在的镜像。

WEB_IMAGE_NAME="shortener-web"
# WEB_IMAGE_NAME：前端镜像的包名。判定条件与写错的后果同 API_IMAGE_NAME。

API_DEPLOYMENT="api"
# API_DEPLOYMENT：后端 Deployment 的名称。
#   要填什么：`kubectl get deploy -n <命名空间>` 中列出的后端工作负载名称。
#   判定条件：与 `40-api.yaml` 中 Deployment 的 metadata.name 一致。
#   写错的后果：取值不一致时，更新镜像与等待 rollout 都会报找不到该 Deployment。

API_CONTAINER="api"
# API_CONTAINER：后端 Deployment 中容器的名称。
#   要填什么：`40-api.yaml` 中 containers 数组里那一项的 name 字段取值。
#   判定条件：容器名称必须与清单中的 name 逐字一致。
#   写错的后果：取值不一致时，`kubectl set image` 报该容器不存在。

WEB_DEPLOYMENT="web"
# WEB_DEPLOYMENT：前端 Deployment 的名称。判定条件与写错的后果同 API_DEPLOYMENT。

WEB_CONTAINER="web"
# WEB_CONTAINER：前端 Deployment 中容器的名称。判定条件与写错的后果同 API_CONTAINER。

API_SERVICE="api"
# API_SERVICE：后端 Service 的名称，供端口转发使用。
#   要填什么：`40-api.yaml` 中 Service 的 metadata.name。
#   判定条件：与清单中的 Service 名称一致。
#   写错的后果：取值不一致时，端口转发无法建立，健康检查阶段一定失败。

API_SERVICE_PORT="8080"
# API_SERVICE_PORT：后端 Service 的端口，也就是端口转发要连接的服务端口。
#   要填什么：`40-api.yaml` 中 Service 的 spec.ports[].port 取值。
#   判定条件：与集群中 Service 的端口一致，而不是容器的 targetPort。
#   写错的后果：取值写成容器的 targetPort 时，端口转发会连到错误的端口并失败。

LOCAL_FORWARD_PORT="8080"
# LOCAL_FORWARD_PORT：本机用于端口转发的端口。
#   要填什么：一个本机未被占用的端口。
#   判定条件：与 API_SERVICE_PORT 区分开，避免与本机其它进程冲突。
#   写错的后果：端口被占用时端口转发启动失败，健康检查阶段随之失败。

HEALTH_PATH="/api/healthz"
# HEALTH_PATH：存活检查端点的路径。
#   要填什么：只确认进程可响应的端点路径，依据是 code/shortener/README.md 第 4 节。
#   判定条件：该端点不检查任何外部依赖。
#   写错的后果：路径写错时返回 404，脚本以非零退出码结束。

READY_PATH="/api/readyz"
# READY_PATH：就绪检查端点的路径。
#   要填什么：会同时确认 PostgreSQL 与 Redis 可用的端点路径。
#   判定条件：该端点在依赖不可用时返回 503。
#   写错的后果：把就绪检查写成存活检查时，无法证明依赖可用。

ROLLOUT_TIMEOUT="30s"
# ROLLOUT_TIMEOUT：等待 rollout 完成的超时上限。
#   要填什么：一个带单位的时间取值，例如 120s。
#   判定条件：必须显式给出，避免 rollout 长时间卡住时无限等待。
#   写错的后果：取值过小时正常滚动更新也可能被判为超时；不设置时会无限等待。

# =============================================================================
# 阶段一：核对当前 context
# =============================================================================
#
# 必须做的事：
#   读取 `kubectl config current-context` 的输出，与 EXPECTED_CONTEXT 比较。
#   不一致时把「当前取值」与「期望取值」都打印出来，并以非零退出码结束。
# 通过标准：
#   context 一致时继续执行；不一致时脚本在任何修改类动作之前结束。
check_context() {
  local current_context
  current_context="$(kubectl config current-context 2>/dev/null || true)"

  if [ "$current_context" != "$EXPECTED_CONTEXT" ]; then
    printf '当前取值: %s\n' "${current_context:-<空>}" >&2
    printf '期望取值: %s\n' "$EXPECTED_CONTEXT" >&2

    exit 1
  fi

  printf 'context 核对通过: %s\n' "$current_context"
}

# =============================================================================
# 阶段二：取得镜像并导入本机 kind 集群的节点
# =============================================================================
#
# 提示一：kind 的节点是 Docker 容器，节点内的 containerd 看不到宿主机 Docker 的镜像库，
#         因此宿主机用 docker build 或 docker pull 得到的镜像必须显式导入节点。
# 提示二：GHCR 中的包默认为私有，取得镜像之前需要先完成 docker login ghcr.io，
#         这一步不属于本脚本的职责，由执行者在运行脚本之前自行完成。
# 必须做的事：
#   用参数中的镜像标签拼接出两个完整镜像名称，取得这两个镜像，再把它们导入集群节点。
# 通过标准：
#   两个完整镜像名称对应的镜像都出现在节点镜像库中。
load_images() {
  local api_image="$1" web_image="$2" image

  for image in "$api_image" "$web_image"; do
    # 标签是完整的提交标识，同样的标签必然对应同样的内容，
    # 因此本地已经有这个标签的镜像时跳过拉取，网络不可用时也能重复执行。
    if docker image inspect "$image" >/dev/null 2>&1; then
      printf '本地已存在该标签的镜像，跳过拉取: %s\n' "$image"
    else
      printf '正在取得镜像: %s\n' "$image"
      docker pull "$image"
    fi
  done

  printf '正在把镜像导入集群 [%s] 的节点\n' "$CLUSTER_NAME"
  kind load docker-image "$api_image" "$web_image" --name "$CLUSTER_NAME"
}

# =============================================================================
# 阶段三：更新两个 Deployment 的容器镜像
# =============================================================================
#
# 必须做的事：
#   更新 API 与 Web 两个 Deployment 中容器的镜像，容器名称取自 API_CONTAINER 与
#   WEB_CONTAINER，不得只更新其中一个。
# 通过标准：
#   `kubectl get deploy -n <命名空间>` 的输出中两个 Deployment 的镜像字段等于新取值。
update_deployment_images() {
  local api_image="$1"
  local web_image="$2"
  
  printf '更新后端镜像: %s\n' "$api_image"
  kubectl set image "deployment/${API_DEPLOYMENT}" "${API_CONTAINER}=${api_image}" -n "$NAMESPACE"

  printf '更新前端镜像: %s\n' "$web_image"
  kubectl set image "deployment/${WEB_DEPLOYMENT}" "${WEB_CONTAINER}=${web_image}" -n "$NAMESPACE"
}

# =============================================================================
# 阶段四：等待 rollout 完成
# =============================================================================
#
# 必须做的事：
#   对两个 Deployment 等待 rollout 完成，并使用 ROLLOUT_TIMEOUT 作为超时上限。
#   任一 rollout 未在超时内完成时，脚本以非零退出码结束。
# 通过标准：
#   两个 Deployment 的 rollout 都返回成功。
wait_for_rollout() {
  local deploy
  for deploy in "$API_DEPLOYMENT" "$WEB_DEPLOYMENT"; do
    printf '等待 Deployment [%s] 完成滚动更新，超时上限 [%s]\n' "$deploy" "$ROLLOUT_TIMEOUT"
    kubectl rollout status "deployment/${deploy}" -n "$NAMESPACE" --timeout="$ROLLOUT_TIMEOUT"
  done
}

# =============================================================================
# 阶段五：通过端口转发检查健康端点
# =============================================================================
#
# 必须做的事：
#   在后台建立到 API_SERVICE 的端口转发，等待转发就绪之后分别请求 HEALTH_PATH 与
#   READY_PATH，取得两个响应码；任一响应码不等于 200 时以非零退出码结束。
#   无论成功还是失败，脚本退出时都必须终止端口转发进程。
# 通过标准：
#   两个端点都返回 200，并且脚本退出时不残留端口转发进程。
# 终止后台端口转发进程。它以 EXIT trap 的形式被调用，因此使用全局变量 pf_pid，
# 并且用 ${pf_pid:-} 处理「尚未赋值」的情形，避免严格模式下的未定义变量错误。
cleanup_port_forward() {
  if [ -n "${pf_pid:-}" ]; then
    kill "$pf_pid" 2>/dev/null || true
  fi
}

check_health() {
  local pf_log="/tmp/deploy-local-port-forward.log"
  local path code i ready=0

  trap 'cleanup_port_forward' EXIT

  # 本地端口被占用时端口转发会启动失败。先给出明确提示，
  # 避免把「隧道没有建立」误判成「应用不健康」。
  if ss -ltnH 2>/dev/null | grep -qE ":${LOCAL_FORWARD_PORT}[[:space:]]"; then
    printf '本地端口 [%s] 已被占用，请修改配置区的 LOCAL_FORWARD_PORT 之后再执行。\n' \
      "$LOCAL_FORWARD_PORT" >&2
    exit 1
  fi

  # 放到后台运行，并用 $! 记录它的进程号
  kubectl -n "$NAMESPACE" port-forward "svc/${API_SERVICE}" \
    "${LOCAL_FORWARD_PORT}:${API_SERVICE_PORT}" >"$pf_log" 2>&1 &
  pf_pid=$!

  # 等待隧道建立。curl 能建立连接说明端口转发已经就绪；
  # 同时检查进程是否仍然存活，进程提前退出时直接把它的日志打印出来。
  for i in $(seq 1 30); do
    if ! kill -0 "$pf_pid" 2>/dev/null; then
      printf '端口转发进程已提前退出，日志内容如下：\n' >&2
      cat "$pf_log" >&2
      exit 1
    fi

    if curl -s -o /dev/null "http://127.0.0.1:${LOCAL_FORWARD_PORT}${HEALTH_PATH}"; then
      ready=1
      break
    fi
    sleep 1
  done

  # 等待超时说明隧道没有建立，这与「隧道正常但端点返回非 200」是两类不同的问题
  if [ "$ready" -ne 1 ]; then
    printf '端口转发在 30 秒内没有就绪，日志内容如下：\n' >&2
    cat "$pf_log" >&2
    exit 1
  fi

  # 逐端点取 HTTP 状态码并判定
  for path in "$HEALTH_PATH" "$READY_PATH"; do
    code="$(curl -s -o /dev/null -w '%{http_code}' \
      "http://127.0.0.1:${LOCAL_FORWARD_PORT}${path}" || true)"
    printf '端点 [%s] 返回 [%s]\n' "$path" "$code"

    if [ "$code" != "200" ]; then
      printf '健康检查失败: 端点 [%s] 返回 [%s]，期望 200\n' "$path" "$code" >&2
      exit 1
    fi
  done
}

image_ref() {
  printf '%s/%s/%s:%s\n' "$REGISTRY" "$IMAGE_NAMESPACE" "$1" "$2"
}

# =============================================================================
# 入口：参数校验与阶段编排
# =============================================================================
main() {
  if [ "$#" -ne 1 ]; then
    printf '用法: %s <镜像标签>\n' "$0" >&2
    exit 2
  fi

  local tag="$1"
  local api_image web_image
  
  api_image="$(image_ref "$API_IMAGE_NAME" "$tag")"
  web_image="$(image_ref "$WEB_IMAGE_NAME" "$tag")"

  printf '\n[阶段一] 核对当前 context\n'
  check_context

  printf '\n[阶段二] 取得镜像并导入 kind 集群的节点\n'
  load_images "$api_image" "$web_image"

  printf '\n[阶段三] 更新两个 Deployment 的容器镜像\n'
  update_deployment_images "$api_image" "$web_image"

  printf '\n[阶段四] 等待滚动更新完成\n'
  wait_for_rollout

  printf '\n[阶段五] 通过端口转发检查健康端点\n'
  check_health

  printf '\n部署与验收完成: 标签 [%s]\n' "$tag"
}

main "$@"