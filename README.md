# Web AI

一个基于 Go + Vue2 的单文件 Web AI 聊天项目：
- 后端单二进制（`./web-ai`）
- 前端通过 `go:embed` 内嵌
- 支持 OpenAI 兼容接口、多模型切换、SQLite 持久化
- Markdown + KaTeX 后端渲染

## 快速开始

1. 准备配置：复制 `config.example.json` 为 `config.json`，填写 `provider.base_url`、`provider.api_key`、`auth.allowed_user_ids`。
2. 构建：

```bash
make build
```

3. 运行：

```bash
./web-ai -config config.json
```

默认访问：`http://localhost:8080`

## Docker 部署

```bash
docker build -t web-ai .

docker run -d \
  --name web-ai \
  -p 3000:8080 \
  -v /path/to/web-ai-data:/data \
  -e CONFIG=/data/config.json \
  web-ai
```

容器持久化目录要求：
- `/data/config.json`
- `/data/web-ai.db`

## 关键说明

- 登录使用 `user_id` 白名单校验（配置于 `auth.allowed_user_ids`）。
- 会话 token 只保存在内存中，不使用 cookie/localStorage。
- `models[].avatar` 可选，不配则按模型 ID 自动映射内置图标。

## 免责声明

**本项目仅用于个人研究与测试。请勿用于生产或商业关键场景。**  
作者不对因使用本项目造成的任何直接或间接问题、数据损失、服务中断或其他后果承担责任。该项目属于实验/玩具性质，请自行评估风险。
