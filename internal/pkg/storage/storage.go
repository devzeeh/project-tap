package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Service interface {
	UploadFile(ctx context.Context, file io.Reader, filename, contentType string) (string, error)
	UploadBase64(ctx context.Context, b64data string) (string, error)
	DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, error)
	DeleteFile(ctx context.Context, key string) error
}

type R2Storage struct {
	client     *s3.Client
	bucketName string
	publicURL  string
}

func NewR2Storage() (Service, error) {
	accountId := os.Getenv("R2_ACCOUNT_ID")
	accessKeyId := os.Getenv("R2_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")
	publicURL := os.Getenv("R2_PUBLIC_URL")

	if accountId == "" || accessKeyId == "" || accessKeySecret == "" || bucketName == "" {
		return nil, fmt.Errorf("missing Cloudflare R2 credentials in environment variables")
	}

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load R2 config: %v", err)
	}

	return &R2Storage{
		client:     s3.NewFromConfig(cfg),
		bucketName: bucketName,
		publicURL:  strings.TrimSuffix(publicURL, "/"),
	}, nil
}

func (s *R2Storage) UploadFile(ctx context.Context, file io.Reader, filename, contentType string) (string, error) {
	key := fmt.Sprintf("documents/%s", filename)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to R2: %v", err)
	}

	// We enforce returning a relative path so the backend can proxy the secure files
	return "/storage/" + key, nil
}

func (s *R2Storage) DownloadFile(ctx context.Context, key string) (io.ReadCloser, string, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve from R2: %v", err)
	}

	contentType := "application/octet-stream"
	if result.ContentType != nil {
		contentType = *result.ContentType
	}
	return result.Body, contentType, nil
}

func (s *R2Storage) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from R2: %v", err)
	}
	return nil
}

func (s *R2Storage) UploadBase64(ctx context.Context, b64data string) (string, error) {
	if b64data == "" {
		return "", nil
	}

	parts := strings.SplitN(b64data, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid base64 data format")
	}

	ext := ".png"
	contentType := "image/png"
	if strings.Contains(parts[0], "application/pdf") {
		ext = ".pdf"
		contentType = "application/pdf"
	} else if strings.Contains(parts[0], "image/jpeg") {
		ext = ".jpg"
		contentType = "image/jpeg"
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 string: %v", err)
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

	return s.UploadFile(ctx, bytes.NewReader(data), filename, contentType)
}
