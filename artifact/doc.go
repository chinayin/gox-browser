// Package artifact 提供统一的产出物存储抽象层。
//
// 支持多种存储后端（本地文件系统、S3/OSS 等），上层调用方通过统一的
// Dir 门面操作产出物，无需关心底层存储细节。Storer 接口是给存储后端
// 实现者的 SPI，消费方永远不直接使用它。
//
// 基本用法:
//
//	st := artifact.New(artifact.NewLocalStore(artifact.LocalConfig{BaseDir: "runtime/artifacts"}))
//	d := st.Dir("task-001")
//	d.Put(ctx, "screenshots/1.png", pngData, map[string]string{"kind": "screenshot"})
//	d.Finalize(ctx)
package artifact
