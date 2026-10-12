# 短链接服务 Kubernetes 项目

本仓库保存短链接服务的应用代码、容器构建文件、Kubernetes 清单和部署实验记录。
日常操作从 `code/shortener/` 开始；本地构建与集群部署步骤见该目录的 README。

## 目录结构

```text
.
├── README.md
├── .gitignore
└── code/
    └── shortener/
        ├── README.md       # 项目总览与本地运行入口
        ├── compose.yaml    # PostgreSQL、Redis、API、Web 的本地联调
        ├── api/            # Go API、测试、Makefile 与 Dockerfile
        ├── web/            # Vue/Vite 前端与 nginx 配置
        └── k8s/
            ├── *.yaml      # 可直接应用的 Kubernetes 清单
            ├── templates/  # 带注释的清单模板，不作为默认部署入口
            └── doc/        # 实验记录、排障材料与历史状态快照
```

## 常用入口

| 任务 | 位置或命令 |
|---|---|
| 阅读项目说明 | `code/shortener/README.md` |
| 后端编译、静态检查和单元测试 | 在 `code/shortener/api/` 执行 `make verify` |
| 前端依赖安装与生产构建 | 在 `code/shortener/web/` 执行 `npm ci && npm run build` |
| 本地依赖与应用联调 | 在 `code/shortener/` 执行 `docker compose up -d --build` |
| 集群部署说明 | `code/shortener/k8s/README.md` |
| 实验与历史证据 | `code/shortener/k8s/doc/` |

`web/dist/` 是前端构建生成目录，不作为源文件维护；依赖版本由 `web/package-lock.json` 锁定。
历史快照只用于回查当时的状态，不代表当前集群状态。
