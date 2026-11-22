package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const (
	spacesPresignTTL = 15 * time.Minute
	// View URLs last long enough to browse an album. The guest page refreshes them before they expire.
	spacesViewTTL = 2 * time.Hour
)

// objectStore is the subset of Spaces used by album routes.
// Browser uploads use presigned PUT; the bucket stays private.
type objectStore interface {
	PresignPut(ctx context.Context, key, contentType string, contentLength int64, expires time.Duration) (string, error)
	PresignGet(ctx context.Context, key, contentType, downloadName string, expires time.Duration) (string, error)
	Head(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, keys ...string) error
}

type spacesClient struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func openSpacesFromEnv(ctx context.Context) (objectStore, error) {
	key := strings.TrimSpace(os.Getenv("SPACES_KEY"))
	secret := strings.TrimSpace(os.Getenv("SPACES_SECRET"))
	bucket := strings.TrimSpace(os.Getenv("SPACES_BUCKET"))
	region := strings.TrimSpace(os.Getenv("SPACES_REGION"))
	endpoint := strings.TrimSpace(os.Getenv("SPACES_ENDPOINT"))
	if key == "" && secret == "" && bucket == "" && region == "" && endpoint == "" {
		return nil, nil
	}
	if key == "" || secret == "" || bucket == "" || region == "" || endpoint == "" {
		return nil, errors.New("set all of SPACES_KEY, SPACES_SECRET, SPACES_BUCKET, SPACES_REGION, and SPACES_ENDPOINT, or leave them all empty")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = false
		// Default checksums are signed into presigned PUTs. Browsers do not send them, so Spaces would reject the upload.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &spacesClient{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  bucket,
	}, nil
}

func (s *spacesClient) PresignPut(ctx context.Context, key, contentType string, contentLength int64, expires time.Duration) (string, error) {
	in := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}
	if contentLength > 0 {
		in.ContentLength = aws.Int64(contentLength)
	}
	req, err := s.presign.PresignPutObject(ctx, in, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *spacesClient) PresignGet(ctx context.Context, key, contentType, downloadName string, expires time.Duration) (string, error) {
	in := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		in.ResponseContentType = aws.String(contentType)
	}
	if downloadName != "" {
		in.ResponseContentDisposition = aws.String(attachmentDisposition(downloadName))
	}
	req, err := s.presign.PresignGetObject(ctx, in, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *spacesClient) Head(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return false, nil
		default:
			return false, err
		}
	}
	return false, err
}

func (s *spacesClient) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if key == "" {
			continue
		}
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
