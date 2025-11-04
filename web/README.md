# LookingGlass Web Frontend

现代化的 LookingGlass 网络诊断 Web 界面。

## 功能特性

- ✅ 实时 Agent 状态展示
- ✅ 支持 Ping、MTR、NextTrace 网络诊断工具
- ✅ 实时命令输出流式显示
- ✅ 历史记录功能（保存在浏览器本地）
- ✅ 现代化响应式界面设计
- ✅ WebSocket + Protobuf 二进制通信

## 技术栈

- **纯 HTML + CSS + JavaScript** (无框架依赖)
- **Protobuf.js** - protobuf 序列化/反序列化
- **WebSocket** - 实时双向通信

## 文件结构

```
web/
├── index.html          # 主页面
├── css/
│   └── style.css       # 样式文件
├── js/
│   ├── protobuf.js     # Protobuf 消息处理
│   ├── websocket.js    # WebSocket 客户端封装
│   └── app.js          # 主应用逻辑
└── README.md           # 本文件
```

## 如何使用

### 方法一：直接通过 HTTP 服务器访问

Master 服务器在启动时会自动在 HTTP 端口（默认 8081）提供静态文件服务。

1. 启动 Master 服务器：
```bash
./bin/master -config master/config.yaml
```

2. 启动至少一个 Agent：
```bash
./bin/agent -config agent/config.yaml
```

3. 在浏览器中打开：
```
http://localhost:8081
```

### 方法二：使用独立 HTTP 服务器

如果你想使用其他 HTTP 服务器，可以：

```bash
# 使用 Python 的简单 HTTP 服务器
cd web
python3 -m http.server 8000

# 或使用 Node.js 的 http-server
npx http-server web -p 8000
```

然后访问 `http://localhost:8000`

**注意**：如果使用独立服务器，需要确保：
- Master WebSocket 服务运行在 `ws://localhost:8080/ws`
- 或修改 `js/app.js` 中的 WebSocket URL

## 配置说明

### WebSocket 连接地址

默认连接到 `ws://localhost:8080/ws`。如果需要修改，编辑 `js/app.js` 中的连接代码：

```javascript
// 在 app.js 的 connect() 方法中
const host = window.location.hostname || 'localhost';
const port = 8080; // 修改这里的端口
```

### Master 配置

确保 Master 的 `config.yaml` 中 WebSocket 端口配置正确：

```yaml
server:
  ws_port: 8080  # WebSocket 端口
  http_port: 8081  # HTTP 静态文件端口
```

### 任务参数配置

前端会使用 Master 配置中的默认参数：

```yaml
task:
  default_ping_count: 4   # Ping 次数
  default_mtr_count: 4    # MTR 次数
```

## 使用流程

1. **连接到服务器**
   - 页面加载后自动连接 WebSocket
   - 右上角显示连接状态

2. **查看 Agent 列表**
   - 左侧面板显示所有 Agent
   - 绿色表示在线，灰色表示离线
   - 显示当前任务数和最大并发数

3. **执行诊断命令**
   - 选择一个在线的 Agent
   - 选择诊断工具（Ping/MTR/NextTrace）
   - 输入目标 IP 或域名
   - 点击 Execute 执行

4. **查看实时输出**
   - 命令输出实时显示在终端区域
   - 自动滚动到最新输出
   - 支持中途取消命令

5. **使用历史记录**
   - 点击 "▶ History" 展开历史
   - 点击历史记录快速重新执行
   - 历史保存在浏览器本地（最多 50 条）

## 浏览器兼容性

支持所有现代浏览器：
- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

需要支持：
- WebSocket
- ES6+ JavaScript
- CSS Grid
- LocalStorage

## 安全说明

1. **生产环境建议**：
   - 使用 HTTPS/WSS 加密连接
   - 配置适当的 CORS 策略
   - 限制访问 IP 白名单

2. **本地存储**：
   - 历史记录保存在浏览器 LocalStorage
   - 不包含敏感信息
   - 可手动清除浏览器缓存删除

## 故障排查

### 无法连接到服务器

1. 检查 Master 是否正在运行
2. 检查 WebSocket 端口（默认 8080）是否可访问
3. 检查浏览器控制台的错误信息
4. 确认防火墙设置

### Agent 列表为空

1. 确保至少有一个 Agent 正在运行
2. 检查 Agent 是否成功注册到 Master
3. 查看 Master 日志确认 Agent 连接状态

### 命令执行失败

1. 确认 Agent 支持该诊断工具
2. 检查目标地址格式是否正确
3. 查看终端输出的错误信息
4. 检查 Agent 日志

## 开发说明

### 修改 Protobuf 定义

如果修改了 `proto/lookingglass.proto`，需要同步更新 `js/protobuf.js` 中的 protobuf schema。

### 调试模式

打开浏览器开发者工具（F12）查看：
- Console: JavaScript 日志和错误
- Network: WebSocket 消息
- Application > Local Storage: 历史记录数据

### 自定义样式

编辑 `css/style.css` 修改界面样式。主要 CSS 变量：

```css
:root {
    --primary-color: #3b82f6;  /* 主色调 */
    --terminal-bg: #0f172a;    /* 终端背景色 */
    /* ... 更多变量 */
}
```

## 未来改进

可能的功能增强：
- [ ] 支持同时执行多个任务
- [ ] 实时 Agent 状态更新（心跳显示）
- [ ] 任务队列管理
- [ ] 导出诊断结果
- [ ] 国际化支持
- [ ] 深色/浅色主题切换
