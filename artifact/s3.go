package artifact

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const s3Timeout = 30 * time.Second

// S3Config S3 协议存储配置 (兼容 AWS S3 / 阿里云 OSS / MinIO)
type S3Config struct {
	// Endpoint S3/OSS endpoint (如 "https://oss-cn-hangzhou.aliyuncs.com")
	Endpoint string

	// Bucket 存储桶名称
	Bucket string

	// Region 区域 (如 "cn-hangzhou", "us-east-1")
	Region string

	// AccessKey 访问密钥 ID
	AccessKey string

	// SecretKey 访问密钥 Secret
	SecretKey string

	// Prefix key 前缀 (如 "artifacts/")，末尾不需要 "/"
	Prefix string
}

// S3Store S3 协议存储
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// 编译期接口合规断言
var _ Storer = (*S3Store)(nil)

// NewS3Store 创建 S3 协议存储
func NewS3Store(cfg S3Config) *S3Store {
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	})

	prefix := cfg.Prefix
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	return &S3Store{
		client: client,
		bucket: cfg.Bucket,
		prefix: prefix,
	}
}

// Upload 上传到 S3/OSS
func (s *S3Store) Upload(ctx context.Context, namespace, subDir, name string, data []byte) (string, error) {
	key := s.prefix + BuildKey(namespace, subDir, name)

	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(DetectContentType(name)),
	})
	if err != nil {
		return "", fmt.Errorf("artifact: s3 upload %q: %w", key, err)
	}

	slog.Debug("artifact: uploaded to s3", "key", key, "size", len(data))
	return key, nil
}

// Download 从 S3/OSS 下载
func (s *S3Store) Download(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()

	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("artifact: s3 download %q: %w", path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("artifact: s3 read %q: %w", path, err)
	}
	return data, nil
}

// Reader 返回流式读取器
//
// 注意：不在内部设置超时，因为返回的 ReadCloser 生命周期由调用方控制。
// 调用方应通过传入带 deadline 的 ctx 来控制超时。
func (s *S3Store) Reader(ctx context.Context, path string) (io.ReadCloser, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("artifact: s3 reader %q: %w", path, err)
	}
	return resp.Body, nil
}
