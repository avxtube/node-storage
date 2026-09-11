package storage

import (
	"context"
	"fmt"
	"path"
	"strings"

	"storage-node/internal/db/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config contains the connection settings needed by an S3 heartbeat.
type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

// PhysicalObjectKey applies the configured S3 prefix to a media key.
func PhysicalObjectKey(storageConfig *models.Storage, key string) string {
	cleanKey := strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(key), "\\", "/"), "/")
	if storageConfig == nil || storageConfig.S3 == nil {
		return cleanKey
	}
	prefix := strings.Trim(storageConfig.S3.Prefix, "/")
	if prefix == "" || cleanKey == prefix || strings.HasPrefix(cleanKey, prefix+"/") {
		return cleanKey
	}
	return path.Join(prefix, cleanKey)
}

func ConfigFromStorage(storageConfig *models.Storage) (S3Config, error) {
	if storageConfig == nil || storageConfig.S3 == nil {
		return S3Config{}, fmt.Errorf("storage has no S3 config")
	}
	result := S3Config{
		Region: storageConfig.S3.Region, Bucket: storageConfig.S3.Bucket,
		AccessKeyID: storageConfig.S3.AccessKeyID, SecretAccessKey: storageConfig.S3.SecretAccessKey,
		ForcePathStyle: storageConfig.S3.ForcePathStyle,
	}
	if storageConfig.S3.Endpoint != nil {
		result.Endpoint = *storageConfig.S3.Endpoint
	}
	return result, nil
}

// GetS3Object opens a media object and forwards an optional HTTP byte range.
func GetS3Object(ctx context.Context, storageConfig *models.Storage, key, byteRange string) (*s3.GetObjectOutput, error) {
	cfg, err := ConfigFromStorage(storageConfig)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	input := &s3.GetObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(PhysicalObjectKey(storageConfig, key))}
	if strings.TrimSpace(byteRange) != "" {
		input.Range = aws.String(byteRange)
	}
	return client.GetObject(ctx, input)
}

// NewS3Client creates an AWS SDK client for any S3-compatible backend.
func NewS3Client(ctx context.Context, config S3Config) (*s3.Client, error) {
	if strings.TrimSpace(config.Bucket) == "" ||
		strings.TrimSpace(config.AccessKeyID) == "" ||
		strings.TrimSpace(config.SecretAccessKey) == "" {
		return nil, fmt.Errorf("missing S3 bucket or credentials")
	}

	region := strings.TrimSpace(config.Region)
	if region == "" || region == "auto" {
		region = "us-east-1"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKeyID,
			config.SecretAccessKey,
			"",
		)),
	}
	if endpoint := normalizeEndpoint(config.Endpoint, config.Bucket); endpoint != "" {
		loadOptions = append(loadOptions, awsconfig.WithBaseEndpoint(endpoint))
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 config: %w", err)
	}

	return s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = config.ForcePathStyle
	}), nil
}

func normalizeEndpoint(endpoint, bucket string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return ""
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}
	if bucket = strings.Trim(strings.TrimSpace(bucket), "/"); bucket != "" && strings.HasSuffix(endpoint, "/"+bucket) {
		endpoint = strings.TrimSuffix(endpoint, "/"+bucket)
	}
	return endpoint
}

// CheckS3Bucket verifies that the configured credentials can reach the bucket.
func CheckS3Bucket(ctx context.Context, client *s3.Client, bucket string) error {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		return fmt.Errorf("head S3 bucket: %w", err)
	}
	return nil
}
