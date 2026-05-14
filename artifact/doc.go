// Package artifact 提供统一的产出物存储抽象层。
//
// 支持多种存储后端（本地文件系统、S3/OSS 等），上层调用方通过统一的
// Storer 接口操作产出物，无需关心底层存储细节。
//
// 基本用法:
//
//	store := artifact.NewLocalStore(artifact.LocalConfig{BaseDir: "runtime/artifacts"})
//	path, err := store.Upload(ctx, "task-001", "step_01", "screenshot.png", pngData)
package artifact
