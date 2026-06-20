# AI快侠 部署指南

本仓库包含 **网页前端**（GitHub Pages）和 **后端 API**（Render 免费托管），可独立对外提供服务。用户量增大后，可按文末步骤迁移到阿里云。

## 架构

```
GitHub 仓库 (ai-kuaixia)
├── landing/          → GitHub Pages（静态网页）
├── backend/          → Render Docker 部署（Go API + yt-dlp）
└── render.yaml       → Render 一键部署配置
```

| 组件 | 托管 | 地址示例 |
|------|------|----------|
| 网页 | GitHub Pages | https://opc007.github.io/aikuaixia/landing/app.html |
| API | Render 免费版 | https://aikuaixia-api.onrender.com |

---

## 第一步：启用 GitHub Pages（网页）

1. 打开仓库 **Settings → Pages**
2. Source 选 **Deploy from a branch**
3. Branch 选 `main`，目录选 **`/ (root)`** 或 **`/landing`**（若根目录有 index.html 用 root）
4. 保存后等待 1～2 分钟，访问：https://opc007.github.io/ai-kuaixia/landing/app.html

> 若自定义域名为 `opc007.github.io/aikuaixia/`，需仓库名为 `aikuaixia`，或在 Pages 设置中配置。

---

## 第二步：部署后端到 Render（API）

1. 注册 [Render](https://render.com)（可用 GitHub 登录）
2. 点击 **New → Blueprint**
3. 连接本仓库 `opc007/ai-kuaixia`
4. Render 会自动读取根目录的 `render.yaml` 并创建服务
5. 等待构建完成（约 5～10 分钟），记下服务 URL，例如：
   ```
   https://aikuaixia-api.onrender.com
   ```

### 环境变量（render.yaml 已预设，可按需修改）

| 变量 | 说明 |
|------|------|
| `DB_DRIVER=sqlite` | 使用 SQLite，无需额外数据库 |
| `DB_PATH=/app/data/aikuaixia.db` | 数据持久化路径（挂载磁盘） |
| `JWT_SECRET` | 自动生成，用于登录 token |
| `GIN_MODE=release` | 生产模式 |

可选：在 Render Dashboard 添加 `MINIMAX_API_KEY` 以启用 AI 对话功能。

---

## 第三步：配置网页 API 地址

部署完成后，编辑 `landing/config.js`：

```javascript
// 方式一：直接指定（推荐）
window.AIKUAIXIA_API_BASE = 'https://aikuaixia-api.onrender.com/api/v1';

// 方式二：修改默认地址
window.AIKUAIXIA_DEFAULT_API = 'https://aikuaixia-api.onrender.com/api/v1';
```

提交并 push 到 GitHub，Pages 会自动更新。

---

## 第四步：验证

1. 打开网页版，注册账号（送 10 积分）
2. 粘贴抖音分享文字 → 解析 → 下载
3. 若 API 冷启动（Render 免费版 15 分钟无访问会休眠），首次请求可能需等待 30～60 秒

---

## Render 免费版限制

| 项目 | 说明 |
|------|------|
| 冷启动 | 15 分钟无流量后休眠，下次访问需等待唤醒 |
| 磁盘 | 1GB 持久化（SQLite 数据） |
| 带宽 | 100GB/月 |
| 构建时间 | 每月有限额 |

**适合**：初期试用、少量用户。  
**不适合**：高并发、7×24 低延迟。

---

## 迁移到阿里云（用户量增大后）

1. **购买轻量应用服务器**（2核2G 起，约 50～100 元/月）
2. **安装依赖**：
   ```bash
   # Docker 方式（推荐）
   git clone https://github.com/opc007/ai-kuaixia.git
   cd ai-kuaixia/backend
   docker build -t aikuaixia-api .
   docker run -d -p 8080:8080 \
     -v aikuaixia-data:/app/data \
     -e DB_DRIVER=sqlite \
     -e JWT_SECRET=你的密钥 \
     aikuaixia-api
   ```
3. **配置域名 + HTTPS**（Nginx + Let's Encrypt）
4. **修改 `landing/config.js`** 指向新域名
5. **（可选）换 PostgreSQL**：设置 `DB_HOST` 等环境变量，去掉 `DB_DRIVER=sqlite`

数据迁移：复制 Render 磁盘上的 `aikuaixia.db` 到新服务器即可。

---

## 本地开发

```bash
# 后端
cd backend
DB_DRIVER=sqlite go run ./cmd/server

# 网页：直接用浏览器打开 landing/app.html，或
cd landing && python3 -m http.server 8081
# 访问 http://localhost:8081/app.html（会自动连 localhost:8080）
```

---

## 常见问题

**Q: 网页能打开，但解析/下载失败？**  
A: 检查 `landing/config.js` 中的 API 地址是否正确，Render 服务是否在线。

**Q: 下载很慢？**  
A: Render 免费版在新加坡节点，国内访问可能较慢，迁移阿里云可改善。

**Q: 不想公开后端源码？**  
A: 可将仓库设为 Private，Render 仍可从私有仓库部署。
