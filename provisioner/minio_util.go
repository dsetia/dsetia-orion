package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "io"

    "orion/common"
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

// DB is the SQLite database handle
type MC struct {
    *minio.Client
}

// InitMinio creates and returns a MinIO client
func NewMinio(cfg common.MinioConfig) (*MC, error) {
    log.Printf("endpoint = %s, accessKey = %s, SecretKey = %s",
        cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
    client, err := minio.New(cfg.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
        Secure: cfg.UseSSL,
    })
    if err != nil {
	log.Printf("failed to init minio client: %w", err)
        return nil, fmt.Errorf("failed to init minio client: %w", err)
    }
    return &MC{client}, nil
}

// UploadObject uploads a file to the specified bucket and object path
func (mc *MC) UploadObject(bucket, objectPath, filePath string) error {
    ctx := context.Background()

    file, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("open file: %w", err)
    }
    defer file.Close()

    info, err := file.Stat()
    if err != nil {
        return fmt.Errorf("stat file: %w", err)
    }

    log.Printf("Uploading %s to bucket %s as %s", filePath, bucket, objectPath)
    _, err = mc.PutObject(ctx, bucket, objectPath, file, info.Size(), minio.PutObjectOptions{})
    if err != nil {
        return fmt.Errorf("upload failed: %w", err)
    }
    return nil
}

// DownloadObject downloads an object to the specified file path
func (mc *MC) DownloadObject(bucket, objectPath, destPath string) error {
    ctx := context.Background()

    reader, err := mc.GetObject(ctx, bucket, objectPath, minio.GetObjectOptions{})
    if err != nil {
        return fmt.Errorf("download failed: %w", err)
    }
    defer reader.Close()

    out, err := os.Create(destPath)
    if err != nil {
        return fmt.Errorf("create dest file: %w", err)
    }
    defer out.Close()

    _, err = io.Copy(out, reader)
    if err != nil {
        return fmt.Errorf("copy error: %w", err)
    }

    return nil
}
