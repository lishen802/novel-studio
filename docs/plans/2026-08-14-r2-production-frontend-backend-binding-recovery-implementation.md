# R2 生产前端后端绑定恢复 Implementation Plan

- 状态：`completed`
- 所属阶段：R2 交互基础回归修复；不改变 R3–R8 产品范围
- 依据：产品需求 5.1、5.3、5.5、5.7，产品设计 3、4、8，系统设计 12

## 目标

保留当前已验收原型的视觉与交互结构，把后端已经存在的项目、章节、变更集、正式资产、版本、导入、作业事件和应用设置能力重新连接到生产 React 界面。后端尚未实现的原创三方向、批次生成和整书档案综合继续显示明确的未就绪状态，不使用 mock 伪造成功。

同时修复桌面顶部重复产品栏：移除独立的第二条产品栏，把模型配置状态入口合入窗口最上方标题区域。

## 范围

- 恢复稳定的 `StudioClient` 与 Wails 运行时适配边界；
- 应用启动读取 `ApplicationBootstrap`，项目状态读取后端 Workspace；
- 接入当前界面已有按钮所覆盖的新建、打开、最近项目、关闭、章节读取/草稿/提交、ChangeSet、正式资产读取与编辑、Snapshot、导入和应用设置；
- AI 任务提交使用真实 AgentRun；当前界面没有任务中心入口，不额外扩张 JobEvent/JobSnapshot 的可视化范围，也不使用计时器模拟进度；
- 保留 fake client 作为前端测试边界；
- 修复 macOS 标题区和生产页面重复顶栏问题，并保持 Windows 可用布局。

## 明确不包含

- 不在本计划实现 R4 三方向与作品方案状态机；
- 不实现 R5 多章批次生成、自动审阅和定稿状态机；
- 不实现 R8 整书档案综合；
- 不增加 REST 服务，不让 React 直接访问 SQLite 或 Provider；
- 不扩展当前项目领域模型中尚不存在的作者署名、封面和目标字数字段。

## 实施顺序（TDD）

1. 为 Wails client、启动状态和结构化错误补充失败测试；
2. 恢复前端 client DTO、运行时调用和应用/工作区 store；
3. 先接首页与项目生命周期，再接正文编辑闭环；
4. 接入当前原型已展示的版本、资产、ChangeSet、导入和设置按钮；
5. 未实现能力改为可解释禁用状态；
6. 修复顶部标题区，并覆盖首页和项目工作区布局测试；
7. 运行前端测试/typecheck/build、Go tests、Wails binding 校验和双平台相关静态检查。

## 数据、兼容与恢复

- 不新增数据库迁移；
- 章节写入继续携带 `revision`，冲突时保留本地编辑并要求刷新；
- 项目打开、只读、lease 冲突和恢复副本沿用现有结构化错误；
- 关闭项目先刷新正文，再调用 `CloseProject`；
- mock 数据不再进入生产运行路径，只允许存在于测试 fake/fixture。

## Windows/macOS

- macOS 使用 Wails v2 的隐藏内嵌标题栏，让 React 顶部控件与交通灯共享窗口顶部区域；
- Windows 保留系统窗口装饰，页面不额外绘制重复产品名称栏；模型入口放在应用内容顶部的紧凑悬浮区；
- 文件选择、项目 lease、只读和路径错误继续由 Go/Wails 处理。

## 验证

```bash
cd apps/desktop/frontend
npm test
npm run typecheck
npm run build

cd ../../../
go test ./...

cd apps/desktop
/Users/shenlee/go/bin/wails generate module
node ../../scripts/ci/verify-wails-bindings.mjs
```

已完成前端测试、TypeScript 类型检查、Vite 生产构建、全量 Go 测试、Wails 绑定生成与校验，以及带 macOS 系统框架链接参数的桌面二进制编译。Wails 自带打包命令在当前 CommandLineTools 环境不会自动链接 `UniformTypeIdentifiers`；使用等价 Go 编译命令显式加入该系统框架后通过，不涉及产品代码故障。
