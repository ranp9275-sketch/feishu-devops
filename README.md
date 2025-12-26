# DevOps 服务

DevOps 服务是一个集成飞书（Feishu/Lark）、Jenkins 和 OA 系统的自动化运维平台。它提供了通过飞书卡片进行应用发布、回滚、重启等操作的能力，并能实时反馈构建状态。

## 功能特性

- **飞书集成**：
    - 支持发送交互式卡片，直接在飞书聊天中进行运维操作。
    - 支持 WebSocket 长连接接收飞书事件回调。
    - 机器人管理 API（增删改查）。
- **Jenkins 集成**：
    - 自动触发 Jenkins 构建任务（Deploy, Gray, Rollback, Restart）。
    - 实时监控构建队列和构建状态。
    - 构建结果（成功/失败/耗时）推送到飞书。
- **发布管理**：
    - 支持灰度发布、正式发布、回滚、重启。
    - 支持批量操作（批量发布、停止批量发布）。
    - 防止重复点击和误操作的保护机制。

## 前置要求

- Go 1.23.3+
- MySQL 5.7+
- Jenkins (需安装相关插件并开启 API Token)
- 飞书开放平台应用 (企业自建应用)

## 配置

复制 `.env.example` 为 `.env` (如果不存在请手动创建) 并设置环境变量：

```bash
# 服务器配置
PORT=8080
LOG_LEVEL=info
READ_TIMEOUT=10s
WRITE_TIMEOUT=10s
SHUTDOWN_TIMEOUT=5s

# 飞书配置
FEISHU_APP_ID=cli_xxxxxxxx          # 飞书应用 App ID
FEISHU_APP_SECRET=xxxxxxxxxxxxxxxx  # 飞书应用 App Secret

# Jenkins 配置
JENKINS_URL=http://your-jenkins-url/
JENKINS_USER=admin
JENKINS_TOKEN=your-jenkins-token

# MySQL 配置
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=your_password
MYSQL_DATABASE=feishu
```

## 运行

### 本地开发

```bash
# 运行服务
make run
# 或
go run main.go

# 运行测试
make test
# 或
go test ./...
```

### 构建

```bash
# 编译二进制文件
make build
# 或
go build -o devops-service main.go

# 构建 Docker 镜像
make docker-build
```

## API 接口文档

所有接口统一前缀：`/app/api/v1`

### 飞书消息相关

- **发送消息/卡片**
    - `POST /feishu/api/send-card`
    - 用于发送文本消息或交互式卡片。

- **版本信息**
    - `GET /feishu/version`

### 机器人管理

- **添加机器人**
    - `POST /robot/addrobot`
- **查询机器人详情**
    - `GET /robot/describe?name=xxx`
- **查询机器人列表**
    - `GET /robot/query`
- **更新机器人**
    - `POST /robot/updaterobot`
- **删除机器人**
    - `POST /robot/delrobot`

### Jenkins 集成

- **测试发布流程**
    - `POST /jk/test-flow`
    - 模拟 OA 推送 -> 生成卡片 -> 发送卡片 -> 触发 Jenkins 的完整流程。

### OA 数据集成

- **存储 JSON 数据**
    - `POST /oa/api/store-json`
    - 接收并存储来自 OA 的原始 JSON 数据。
- **获取指定 JSON 数据**
    - `GET /oa/api/get-json/:id`
- **获取所有 JSON 数据**
    - `GET /oa/api/get-json-all`
- **获取最新 JSON 数据**
    - `GET /oa/api/get-latest-json`

### 系统接口

- **健康检查**
    - `GET /health`
- **Prometheus 指标**
    - `GET /metrics`

## 项目结构

```
.
├── feishu/
│   ├── config/         # 配置加载
│   ├── pkg/
│   │   ├── feishu/     # 飞书 SDK 封装
│   │   ├── handler/    # 飞书消息/卡片处理器 (核心业务逻辑)
│   │   ├── reg/        # 服务注册与健康检查
│   │   └── robot/      # 机器人管理模块
├── jenkins/
│   ├── oa-jenkins/     # Jenkins 与 OA/Feishu 的集成逻辑
│   └── jenkins.go      # Jenkins 客户端封装
├── oa/                 # OA 系统集成 (Job 数据获取)
├── tools/              # 通用工具 (IOC, Logger, Middleware)
├── main.go             # 程序入口
└── README.md           # 说明文档
```
