package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	stashconfig "github.com/codecflow/fabric/stash/config"
)

const (
	defaultDownloadConcurrency = 16
	defaultDownloadChunkSize   = 64 * 1024 * 1024 // 64MB
)

// S3Config represents S3 source configuration.
type S3Config struct {
	BucketName  string
	Region      string
	EndpointURL string
	AccessKey   string
	SecretKey   string
}

// S3Client handles S3 operations with concurrent chunked downloads.
type S3Client struct {
	client              *s3.Client
	source              S3Config
	downloadConcurrency int64
	downloadChunkSize   int64
}

// NewS3Client creates a new S3 client.
func NewS3Client(ctx context.Context, sourceConfig S3Config, serverConfig *stashconfig.Server) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(sourceConfig.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			sourceConfig.AccessKey,
			sourceConfig.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if sourceConfig.EndpointURL != "" {
		cfg.BaseEndpoint = aws.String(sourceConfig.EndpointURL)
	}

	downloadConcurrency := serverConfig.S3DownloadConcurrency
	if downloadConcurrency <= 0 {
		downloadConcurrency = defaultDownloadConcurrency
	}

	downloadChunkSize := serverConfig.S3DownloadChunkSize
	if downloadChunkSize <= 0 {
		downloadChunkSize = defaultDownloadChunkSize
	}

	s3Client := s3.NewFromConfig(cfg)
	return &S3Client{
		client:              s3Client,
		source:              sourceConfig,
		downloadConcurrency: downloadConcurrency,
		downloadChunkSize:   downloadChunkSize,
	}, nil
}

// Head performs a HEAD request to check if object exists and get metadata.
func (c *S3Client) Head(ctx context.Context, key string) (bool, *s3.HeadObjectOutput, error) {
	output, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.source.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil, err
	}

	return true, output, nil
}

type chunkResult struct {
	index int
	data  []byte
	err   error
}

// DownloadIntoBuffer downloads an S3 object into a buffer using concurrent chunked downloads.
func (c *S3Client) DownloadIntoBuffer(ctx context.Context, key string, buffer *bytes.Buffer) error {
	ok, head, err := c.Head(ctx, key)
	if err != nil || !ok {
		return err
	}

	size := aws.ToInt64(head.ContentLength)
	if size <= 0 {
		return fmt.Errorf("invalid object size: %d", size)
	}

	numChunks := int((size + c.downloadChunkSize - 1) / c.downloadChunkSize)
	chunkCh := make(chan chunkResult, numChunks)
	sem := make(chan struct{}, c.downloadConcurrency)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < numChunks; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := int64(i) * c.downloadChunkSize
			end := start + c.downloadChunkSize - 1
			if end >= size {
				end = size - 1
			}

			rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)
			resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(c.source.BucketName),
				Key:    aws.String(key),
				Range:  &rangeHeader,
			})
			if err != nil {
				chunkCh <- chunkResult{i, nil, fmt.Errorf("range request failed for %s: %w", rangeHeader, err)}
				cancel()
				return
			}
			defer resp.Body.Close()

			part := make([]byte, end-start+1)
			n, err := io.ReadFull(resp.Body, part)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				chunkCh <- chunkResult{i, nil, fmt.Errorf("error reading range %s: %w", rangeHeader, err)}
				cancel()
				return
			}
			chunkCh <- chunkResult{i, part[:n], nil}
		}(i)
	}

	go func() {
		wg.Wait()
		close(chunkCh)
	}()

	chunks := make([][]byte, numChunks)
	var errs []error
	for res := range chunkCh {
		if res.err != nil {
			errs = append(errs, res.err)
		}
		chunks[res.index] = res.data
	}
	if len(errs) > 0 {
		return fmt.Errorf("download errors: %v", errs)
	}

	buffer.Reset()
	for _, chunk := range chunks {
		buffer.Write(chunk)
	}
	return nil
}
