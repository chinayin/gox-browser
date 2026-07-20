package artifact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

const s3Timeout = 30 * time.Second

// S3Config S3 协议存储配置 (兼容 AWS S3 / 阿里云 OSS / MinIO)
type S3Config struct {
	Endpoint  string // 如 "https://oss-cn-hangzhou.aliyuncs.com"
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Prefix    string // key 前缀，末尾无需 "/"
}

// S3Store S3 协议存储
type S3Store struct {
	client *s3.Client
	// uploader 用于流式分片上传大对象。
	// manager.Uploader 已被标记 deprecated（建议迁移到 feature/s3/transfermanager），
	// 但截至本次实现 transfermanager 仍是 v0.x 预览版（无稳定 API 保证），
	// 而 manager 是 v1 稳定包，故按需求继续使用；待 transfermanager 转正后再迁移。
	uploader *manager.Uploader //nolint:staticcheck // SA1019: 见上，transfermanager 未到 v1 稳定版前暂缓迁移
	presign  *s3.PresignClient
	bucket   string
	prefix   string
}

var _ Storer = (*S3Store)(nil)

// NewS3Store 创建 S3 协议存储
func NewS3Store(cfg S3Config) *S3Store {
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true, // MinIO / 自定义 endpoint 兼容
	})
	prefix := cfg.Prefix
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return &S3Store{
		client:   client,
		uploader: manager.NewUploader(client), //nolint:staticcheck // SA1019: 见字段注释
		presign:  s3.NewPresignClient(client),
		bucket:   cfg.Bucket,
		prefix:   prefix,
	}
}

func (s *S3Store) fullKey(key string) string { return s.prefix + key }

func (s *S3Store) Put(ctx context.Context, key string, data []byte, contentType string) error {
	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.fullKey(key)),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("artifact: s3 put %q: %w", key, err)
	}
	slog.Debug("artifact: put to s3", "key", key, "size", len(data))
	return nil
}

func (s *S3Store) PutReader(ctx context.Context, key string, r io.Reader, contentType string) error {
	// 不设内部超时：大对象流式上传时长不定，由调用方经 ctx 控制。Uploader 自动多分片。
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{ //nolint:staticcheck // SA1019: 见字段注释
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.fullKey(key)),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("artifact: s3 upload %q: %w", key, err)
	}
	return nil
}

func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.fullKey(key))})
	if err != nil {
		if isS3NotFound(err) {
			return nil, fmt.Errorf("artifact: s3 get %q: %w", key, ErrNotFound)
		}
		return nil, fmt.Errorf("artifact: s3 get %q: %w", key, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("artifact: s3 read %q: %w", key, err)
	}
	return data, nil
}

func (s *S3Store) Reader(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.fullKey(key))})
	if err != nil {
		if isS3NotFound(err) {
			return nil, fmt.Errorf("artifact: s3 reader %q: %w", key, ErrNotFound)
		}
		return nil, fmt.Errorf("artifact: s3 reader %q: %w", key, err)
	}
	return resp.Body, nil
}

func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s3Timeout)
	defer cancel()
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.fullKey(key))})
	if err != nil {
		if isS3NotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("artifact: s3 head %q: %w", key, err)
	}
	return true, nil
}

func (s *S3Store) SignURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.fullKey(key)),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("artifact: s3 presign %q: %w", key, err)
	}
	return req.URL, nil
}

// isS3NotFound 判断错误是否为对象不存在（GetObject→NoSuchKey / HeadObject→NotFound / 404）。
func isS3NotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
