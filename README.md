# IAM 项目部署

本目录通过 Docker Compose 编排 IAM 系统的完整运行环境，共包含五个容器服务。

## 架构

```
                    ┌─────────────────────────────────┐
                    │        iam-nginx (外层 nginx)    │
                    │     监听 80 / 443 端口            │
                    │     HTTPS 入口 + 路由分发          │
                    └──────────┬──────────┬───────────┘
                               │          │
                    /api/*     │          │  /*
                               ▼          ▼
              ┌────────────────────┐  ┌─────────────────────┐
              │   iam-backend      │  │   iam-frontend      │
              │   Go API 服务      │  │   React SPA          │
              │   内部端口 8080     │  │   内部 nginx 托管    │
              └───────┬───────┬────┘  └─────────────────────┘
                      │       │
                      │       │
            ┌─────────┘       └─────────┐
            ▼                           ▼
   ┌──────────────────┐      ┌──────────────────┐
   │    iam-mysql     │      │    iam-redis     │
   │    MySQL 8       │      │    Redis 7      │
   │    端口 3306     │      │    端口 6379     │
   └──────────────────┘      └──────────────────┘
```

### 服务说明

| 服务 | 镜像 | 容器内端口 | 对外端口 | 用途 |
|------|------|-----------|---------|------|
| **nginx** | nginx:alpine | 80 / 443 | 80 / 443 | 统一入口，HTTPS 终止 + 反向代理 |
| **frontend** | 自建（React + 内部 nginx） | 80 | 无 | 前端 SPA 静态资源托管 |
| **backend** | 自建（Go 多阶段构建） | 8080 | 无 | IAM 后端 API |
| **mysql** | mysql:8 | 3306 | 3306 | 业务数据持久化 |
| **redis** | redis:7-alpine | 6379 | 6379 | session / token / 缓存 / 限流 |

### 数据流

- `HTTP/HTTPS` → **外层 nginx**
  - `/api/*` → **backend:8080**
  - `/*` → **frontend:80**（内部 nginx 做 SPA 路由兜底）
- **backend** ↔ **mysql**（读写业务数据）
- **backend** ↔ **redis**（缓存、会话）

### 网络与存储

- 所有容器在 `iam-net` 自定义 bridge 网络内，容器间通过服务名直接访问。
- MySQL 数据持久化到 named volume `mysql-data`。
- Redis 数据持久化到 named volume `redis-data`。
- 证书文件存放在 `./nginx/certs/`，以只读方式挂载进 nginx 容器。

## 目录结构

```
deployments/
├── docker-compose.yml       # 编排文件
├── README.md
└── nginx/
    ├── nginx.conf           # 外层 nginx 配置（HTTPS + 路由）
    └── certs/
        ├── server.crt       # 自签名证书
        └── server.key       # 私钥
```

## 前置条件

- Docker >= 24.0
- Docker Compose v2
- 已克隆完整仓库（iam-go / iam-web 目录与 deployments 同级）

## 启动

```bash
# 进入 deployments 目录
cd deployments

# 构建镜像并后台启动所有服务
docker compose up -d --build
```

首次启动需要编译 Go 后端和 React 前端，耗时较长。可用以下命令查看状态：

```bash
# 查看所有容器状态
docker compose ps

# 查看实时日志
docker compose logs -f

# 只看某个服务日志
docker compose logs -f backend
```

## 访问

### 本地开发

在系统 hosts 文件中添加（管理员权限）：

```
C:\Windows\System32\drivers\etc\hosts
```

```
127.0.0.1  iam.example.com
```

然后浏览器访问：**https://iam.example.com**

> 自签名证书浏览器会提示不受信任，点击 **高级 → 继续访问** 即可。

### 常用操作

```bash
# 停止所有服务（保留数据卷）
docker compose down

# 停止并删除数据卷（⚠️ 数据库数据将丢失）
docker compose down -v

# 单独重启某个服务
docker compose restart backend

# 重新构建并启动某个服务
docker compose up -d --build backend
```

## 自签名证书说明

证书位置：`nginx/certs/`，有效期 10 年，CN 为 `iam.example.com`，SAN 包含：

- `iam.example.com`
- `localhost`
- `127.0.0.1`

如需重新生成，在 `certs/` 目录执行：

```bash
openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
  -keyout server.key -out server.crt \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=IAM/OU=Dev/CN=iam.example.com" \
  -addext "subjectAltName=DNS:iam.example.com,DNS:localhost,IP:127.0.0.1"
```

## 注意事项

- MySQL 初始化需要几秒才能 accept connections，backend 可能会报连接失败，等待几秒后自动重试即可（`restart: unless-stopped`）。
- 当前 MySQL root 密码为 `root`，生产环境请务必修改 `MYSQL_ROOT_PASSWORD`。
- Redis 未设置密码，生产环境请加上 `requirepass` 配置。
- 当前证书为自签名，生产环境应替换为真实 CA 签发的证书（如 Let's Encrypt）。