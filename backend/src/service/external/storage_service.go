package external

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type StorageService struct {
	client     *s3.Client
	bucket     string
	projectURL string
}

func NewStorageService(endpoint, accessKey, secretKey, region, bucket, projectURL string) *StorageService {
	client := s3.NewFromConfig(aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
	})
	return &StorageService{client: client, bucket: bucket, projectURL: projectURL}
}

func (s *StorageService) Upload(ctx context.Context, folder string, file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := filepath.Ext(header.Filename)
	key := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(header.Header.Get("Content-Type")),
	})
	if err != nil {
		return "", fmt.Errorf("upload to storage: %w", err)
	}

	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.projectURL, s.bucket, key), nil
}
