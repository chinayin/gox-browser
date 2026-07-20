package artifact

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
)

func TestNewS3Store_PrefixHandling(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		wantPrefix string
	}{
		{
			name:       "prefix with trailing slash stays unchanged",
			prefix:     "artifacts/",
			wantPrefix: "artifacts/",
		},
		{
			name:       "prefix without trailing slash gets one appended",
			prefix:     "artifacts",
			wantPrefix: "artifacts/",
		},
		{
			name:       "empty prefix stays empty",
			prefix:     "",
			wantPrefix: "",
		},
		{
			name:       "nested prefix without slash",
			prefix:     "data/scraper",
			wantPrefix: "data/scraper/",
		},
		{
			name:       "nested prefix with slash",
			prefix:     "data/scraper/",
			wantPrefix: "data/scraper/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewS3Store(S3Config{
				Endpoint:  "https://s3.example.com",
				Bucket:    "test-bucket",
				Region:    "us-east-1",
				AccessKey: "test-key",
				SecretKey: "test-secret",
				Prefix:    tt.prefix,
			})

			assert.Equal(t, tt.wantPrefix, store.prefix)
		})
	}
}

// TestS3Store_FullKey 断言 fullKey 正确拼接 prefix，覆盖 NewS3Store_PrefixHandling
// 未直接验证的 fullKey 行为。
func TestS3Store_FullKey(t *testing.T) {
	s := NewS3Store(S3Config{
		Endpoint: "https://s3.example.com", Bucket: "b", Region: "us-east-1",
		AccessKey: "ak", SecretKey: "sk", Prefix: "artifacts",
	})
	assert.Equal(t, "artifacts/task-1/a.json", s.fullKey("task-1/a.json"))
}

// TestS3Store_IsNotFound 用合成错误离线断言 isS3NotFound 的判定逻辑，
// 因为默认 CI 中 S3 集成测试会被 Skip，这条路径原本完全没有覆盖。
func TestS3Store_IsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "NoSuchKey 类型", err: &types.NoSuchKey{}, want: true},
		{name: "NotFound 类型", err: &types.NotFound{}, want: true},
		{name: "GenericAPIError NoSuchKey", err: &smithy.GenericAPIError{Code: "NoSuchKey"}, want: true},
		{name: "GenericAPIError NotFound", err: &smithy.GenericAPIError{Code: "NotFound"}, want: true},
		{name: "GenericAPIError AccessDenied", err: &smithy.GenericAPIError{Code: "AccessDenied"}, want: false},
		{name: "普通 error", err: errors.New("boom"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isS3NotFound(tt.err))
		})
	}
}

// TestS3Store_SignURLOffline 测试 SignURL 的离线 presigner 功能。
func TestS3Store_SignURLOffline(t *testing.T) {
	s := NewS3Store(S3Config{
		Endpoint: "https://s3.example.com", Bucket: "b", Region: "us-east-1",
		AccessKey: "ak", SecretKey: "sk", Prefix: "artifacts",
	})
	url, err := s.SignURL(context.Background(), "task-1/envelope.json", 15*time.Minute)
	if err != nil {
		t.Fatalf("SignURL: %v", err)
	}
	if !strings.Contains(url, "task-1/envelope.json") || !strings.Contains(url, "X-Amz-Signature") {
		t.Fatalf("presigned url 不含预期字段: %s", url)
	}
}

// TestS3Store_Integration 需要一个 S3 兼容端点（如本地 MinIO）。
// 设置这些环境变量后运行：
//
//	ARTIFACT_S3_TEST_ENDPOINT, ARTIFACT_S3_TEST_BUCKET,
//	ARTIFACT_S3_TEST_AK, ARTIFACT_S3_TEST_SK (Region 默认 us-east-1)
func TestS3Store_Integration(t *testing.T) {
	ep := os.Getenv("ARTIFACT_S3_TEST_ENDPOINT")
	if ep == "" {
		t.Skip("未设 ARTIFACT_S3_TEST_ENDPOINT，跳过 S3 集成测试")
	}
	s := NewS3Store(S3Config{
		Endpoint:  ep,
		Bucket:    os.Getenv("ARTIFACT_S3_TEST_BUCKET"),
		Region:    "us-east-1",
		AccessKey: os.Getenv("ARTIFACT_S3_TEST_AK"),
		SecretKey: os.Getenv("ARTIFACT_S3_TEST_SK"),
		Prefix:    "artifacts-test",
	})
	ctx := context.Background()

	if ok, err := s.Exists(ctx, "task-x/missing"); err != nil || ok {
		t.Fatalf("Exists missing = %v,%v", ok, err)
	}
	if err := s.Put(ctx, "task-x/a.json", []byte(`{"x":1}`), "application/json"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if ok, err := s.Exists(ctx, "task-x/a.json"); err != nil || !ok {
		t.Fatalf("Exists after Put = %v,%v", ok, err)
	}
	got, err := s.Get(ctx, "task-x/a.json")
	if err != nil || string(got) != `{"x":1}` {
		t.Fatalf("Get = %q,%v", got, err)
	}
	if err := s.PutReader(ctx, "task-x/big.txt", strings.NewReader("streamed"), "text/plain"); err != nil {
		t.Fatalf("PutReader: %v", err)
	}
	rc, err := s.Reader(ctx, "task-x/big.txt")
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "streamed" {
		t.Fatalf("Reader = %q", b)
	}
	if _, err := s.Get(ctx, "task-x/missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing err = %v, want ErrNotFound", err)
	}
}
