package artifact

import "errors"

var (
	// ErrNotFound 表示产出物不存在
	ErrNotFound = errors.New("artifact: not found")

	// ErrUploadFailed 表示上传失败
	ErrUploadFailed = errors.New("artifact: upload failed")

	// ErrSignURLUnsupported 表示该后端不支持签发直连地址（如本地存储）
	ErrSignURLUnsupported = errors.New("artifact: sign url unsupported by backend")
)
