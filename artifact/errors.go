package artifact

import "errors"

var (
	// ErrNotFound 表示产出物不存在
	ErrNotFound = errors.New("artifact: not found")

	// ErrUploadFailed 表示上传失败
	ErrUploadFailed = errors.New("artifact: upload failed")
)
