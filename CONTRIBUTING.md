# 贡献指南

欢迎为 GinChat 贡献代码！

## 如何贡献

### 提 Bug

1. 搜索 [Issues](https://github.com/yuda-bai/IM-base/issues) 确认没有重复
2. 新建 Issue，使用 Bug Report 模板
3. 描述：期望行为 vs 实际行为，复现步骤，环境信息

### 提 Feature

1. 先开 Issue 讨论需求和设计方案
2. 达成共识后再提交 PR

### 提交 PR

1. Fork 本仓库
2. 创建分支：`git checkout -b feat/your-feature`
3. 提交代码，遵循现有代码风格
4. 确保构建通过：`go build . && cd frontend && npm run build`
5. 提交 PR 到 `master` 分支

## 开发规范

### Go 后端

- 遵循 Go 标准代码风格（`gofmt`）
- 新增 API 接口需添加 Swagger 注解
- 数据库操作使用 GORM，避免原生 SQL
- 打印日志使用 `fmt.Println`（当前阶段）

### Vue 前端

- 使用 Composition API (`<script setup>`)
- 状态管理使用 Pinia
- API 调用统一在 `src/api/` 目录封装
- 组件放在 `src/components/`，页面放在 `src/views/`

### Commit 规范

```
feat: 新功能
fix: 修复 Bug
docs: 文档更新
refactor: 重构
perf: 性能优化
test: 测试
chore: 构建/工具
```

## 本地开发

```bash
# 后端
go run main.go

# 前端
cd frontend && npm run dev

# 访问 http://localhost:5173
```

## 许可证

贡献的代码将采用 MIT 许可证，详见 [LICENSE](LICENSE)。
