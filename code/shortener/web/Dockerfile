# 前端镜像采用两阶段构建：
#   第一阶段（build）使用 Node 镜像安装依赖并且执行 vite build，产出静态文件目录 dist/；
#   第二阶段（runtime）使用 nginx 镜像，只把 dist/ 的产物与 nginx.conf 复制进去。
# 这样做的结果是最终镜像里既没有 Node 运行时，也没有 node_modules 与源码，镜像体积显著变小，
# 攻击面也随之变小。这与第 1 周练习的多阶段构建是同一个模式。

# ---------- 第一阶段：构建 ----------
FROM node:22-alpine AS build

WORKDIR /app

# 先只复制依赖清单，再执行安装。这样只要依赖清单没有变化，
# 这一层就会被 Docker 的构建缓存命中，修改源码时不需要重新下载依赖。
COPY package.json package-lock.json* ./

# 存在 package-lock.json 时执行 npm ci，可以得到与锁文件完全一致的依赖版本；
# 锁文件不存在时（例如你还没有执行过 npm install），退回到 npm install 生成一份。
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund

# 复制其余源码，其中 node_modules 与 dist 由 .dockerignore 排除，不会被带进镜像上下文。
COPY . .

RUN npm run build


# ---------- 第二阶段：运行 ----------
FROM nginx:1.27-alpine AS runtime

# 覆盖镜像自带的默认站点配置，改成同时提供静态文件与反向代理后端的那一份。
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 只把构建产物复制进静态文件根目录，第一阶段安装的依赖与源码都不会进入最终镜像。
COPY --from=build /app/dist /usr/share/nginx/html

# 容器内监听的端口是 80。映射到宿主机或集群的端口由 Compose 的 ports 字段
# 与 Kubernetes 的 Service 字段决定，本文件不参与决定。
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
