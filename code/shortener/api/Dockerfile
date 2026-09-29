# 后端镜像采用两阶段构建，与 web/Dockerfile 是同一个模式。

# ---------- 第一阶段：编译 ----------
# 基础镜像的版本必须不低于 go.mod 中 go 指令声明的版本，
# 否则镜像构建时会直接报「go.mod requires go >= 1.25.0」并且失败。
# 当前 go.mod 中的声明是 go 1.25.0，它由依赖 pgx 与 go-redis 的最低版本要求决定，
# 因此这里使用 golang:1.25-alpine。升级依赖之后需要回来确认这一行。
FROM golang:1.25-alpine AS build

WORKDIR /src

# 先只复制依赖清单再下载依赖。只要这两个文件没有变化，
# 这一层就会被构建缓存命中，修改源码时不需要重新下载依赖。
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 关闭对 C 语言运行库的依赖，产出静态链接的可执行文件，
# 因此第二阶段可以使用体积很小的基础镜像。
# -trimpath 去掉可执行文件中记录的宿主机源码路径，避免把构建环境的目录结构带进镜像。
# -ldflags="-s -w" 去掉符号表与调试信息，可以明显减小文件体积；
# 代价是用 pprof 做符号化分析时会缺少信息，本项目的排障不依赖这一项。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---------- 第二阶段：运行 ----------
FROM alpine:3.20 AS runtime

# ca-certificates 提供根证书，tzdata 提供时区数据库。
# 本项目的时间字段全部由 PostgreSQL 的 now() 生成并以 timestamptz 形式取出，
# 安装 tzdata 是为了让日志时间可以按本地时区输出。
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 appuser

COPY --from=build /out/api /usr/local/bin/api

# 使用非特权用户运行。容器默认以 root 运行，一旦进程被攻破，攻击者获得的是容器内的 root 身份；
# 切换到普通用户可以限制这一步的影响范围。到了第 4 周，Kubernetes 侧的
# securityContext 与这里的 USER 指令是同一件事的两个层次，两处都要设置。
USER 10001

# 把 gin 的运行模式设置为 release。
# gin 的默认取值是 debug，debug 模式下它在启动时打印全部路由清单，并且关闭一部分优化。
# 路由清单属于启动期的一次性信息，而容器中的日志会被采集系统收集，
# 因此这里显式设置为 release；本地联调时可以用环境变量覆盖为 debug 来查看路由清单。
ENV GIN_MODE=release

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]

# 关于基础镜像的选择，这里刻意保留了 shell：本镜像基于 alpine，因此 kubectl exec 进去之后
# 可以使用 sh、nslookup、ps 这些命令，这对第 3 周与第 4 周的排障练习非常重要。
# 如果换成 gcr.io/distroless/static 这类不含 shell 的镜像，镜像会更小、攻击面会更小，
# 但是 kubectl exec 会直接报无法启动 shell，排障时只能依赖日志与探针。
# 这是一个明确的取舍，你在面试中应当能够说明两种选择的差别。
