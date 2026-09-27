# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

用户明确指定（本次前端重建）：`web/` 目录从零搭建，Vue 3 + TypeScript + Pinia + Tailwind CSS v4 + Vite。硬约束：不引入复杂第三方组件库，底层组件全部手写；旧前端目录已删除，不作为参考。Go 后端与 WebSocket 协议约定保持不变，前端先以 mock 数据跑通 UI。

## Users

个人开发者自托管。在自己的机器 / 局域网 / 服务器上运行单二进制，用手机或电脑浏览器访问，基本是自己用。界面第一语言为中文。

## Product Purpose

Licode 是自托管的 AI 编程助手：单 Go 二进制内嵌全部前端，浏览器打开即用。成功标准是「拷贝即部署、内网/离线开箱即用」的同时提供完整的 Agent 工作台能力。

## Positioning

两条主张并重，缺一不可：
1. 单二进制 + 前端产物自包含，无任何外网 CDN 依赖，内网/离线环境可直接部署；
2. 完整 Agent 能力深度：多厂商原生接口、工具调用审批、子代理并行调度、MCP/Skills、沙箱执行。
同类自托管 chat 前端通常只有其一。

## Operating Context

自托管于用户自有环境；局域网/手机访问；可选登录认证；所有数据存于 `~/.licode`（配置、会话、密钥），数据本地化是产品承诺的一部分。

## Capabilities and Constraints

- Go 后端 HTTP/WebSocket 接口约定保持不变（前端重建期间先 mock）。
- 前端产物必须离线自包含，禁止运行时外网请求（含字体/图标 CDN）。
- 产品名 Licode 锁定。
- 中文为界面第一语言；多语言 i18n 暂无硬需求（未决）。
- 不使用全局可变状态（后端既有教训，前端同样遵循：状态收敛进 Pinia store）。

## Brand Commitments

- 名称「Licode」固定，语音中性、工具感。
- 视觉方向由用户明确指定为 DeepSeek Harness 风格：暗黑、三栏、细边框、毛玻璃。本次为替换式重建，旧视觉不再作为约束。

## Evidence on Hand

- `README.md` 功能清单、`docs/` 下的架构/API/协议文档。
- 无 testimonial、无客户案例、无基准数据。未来工作不得虚构此类内容。

## Product Principles

1. 离线自包含是不可妥协的底线，任何依赖必须先问「断网还能跑吗」。
2. 界面服务于任务：工具调用、审批、会话状态必须一眼可扫，装饰让位于可扫描性。
3. 单二进制的简单性是卖点，前端复杂度不得侵蚀「拷贝即用」。
4. 数据属于用户本机；任何采集/外发行为默认不存在。
