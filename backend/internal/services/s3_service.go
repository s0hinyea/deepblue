// internal/services/s3_service.go
//
// Phase 4 — Spec 4.1: S3 Upload Service
package services

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadImage uploads raw image bytes to S3 under the given fileName and
// returns the public HTTPS URL of the object.
//
// Required env vars: S3_BUCKET_NAME, AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
func UploadImage(ctx context.Context, fileData []byte, fileName string) (string, error) {
	bucket := os.Getenv("S3_BUCKET_NAME")
	if bucket == "" {
		return "", fmt.Errorf("S3_BUCKET_NAME is not set")
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &fileName,
		Body:   bytes.NewReader(fileData),
	})
	if err != nil {
		return "", fmt.Errorf("S3 PutObject: %w", err)
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, fileName)
	return url, nil
}
