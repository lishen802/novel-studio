# 织境 Story Studio 文档索引

本目录只保留当前产品的权威文档、仍有效的架构决策和运行保障资料。此前围绕旧产品定位产生的需求、原型设计与实施计划已移除；需要追溯时请查看 Git 历史，不应再将其作为实施依据。

## 阅读顺序

1. [项目交接](handoff/START-HERE.md)：仓库现状、工作约束和进入点；
2. [产品需求](product/requirements.md)：完整功能范围和验收标准；
3. [产品与交互设计](product/design.md)：作者工作台、阶段和确认交互；
4. [系统技术设计](architecture/system-design.md)：架构、数据、运行时和交付约束；
5. [总体开发路线图](roadmap/development-roadmap.md)：阶段依赖、实现顺序和退出标准；
6. [架构决策](architecture/)：仍有效的技术基线与目录边界；
7. [实施计划](plans/)：当前具体能力的可执行任务与验证；
8. [自动化测试基线](testing/automated-test-baseline.md)、[Wails 双平台构建矩阵](testing/wails-build-matrix.md)、[项目恢复](operations/project-recovery.md)与[基础手工验收](testing/foundation-acceptance.md)：自动门禁、构建产物、运行保障和双平台人工验收。

## 文档优先级

当文档存在表述差异时，优先级为：产品需求 → 产品与交互设计 → 系统技术设计 → 架构决策 → 总体开发路线图 → 实施计划 → 验收与运维文档。路线图和实施计划只安排执行顺序，不能改变上游产品与架构决定；实现、原型或历史提交也不能覆盖这些明确决定。

产品功能、交互和技术设计发生变更时，应同步更新上述三份权威文档、总体路线图及受影响的验收项；不要重新引入孤立的“临时需求”“旧原型设计”或未绑定当前需求与路线图阶段的实施计划。
