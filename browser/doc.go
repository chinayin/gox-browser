// Package browser 提供统一的浏览器池抽象层。
//
// 支持多种浏览器后端（Rod、Browserless、Surf 等），
// 上层调用方通过统一的 Browser 接口操作页面，无需关心底层实现。
//
// 架构参考 database/sql 的 Strategy Pattern：
//   - 父包定义接口（Browser、Provider）和通用能力（Pool、Fetcher）
//   - 子包（rod/、browserless/、surf/）提供具体实现
//   - 用户按需 import 子包，配置驱动注册
package browser
