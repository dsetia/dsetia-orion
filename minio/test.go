package main

import (
    "context"
    "flag"
    "log"
    "os"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
    // Define command-line flags
    bucket := flag.String("bucket", "", "MinIO bucket name")
    file := flag.String("file", "", "File path to upload")
    flag.Parse()

    // Validate arguments
    if *bucket == "" || *file == "" {
        log.Println("Usage: go run test.go -bucket <bucket-name> -file <file-path>")
        flag.PrintDefaults()
        os.Exit(1)
    }

    // Initialize MinIO client
    ctx := context.Background()
    client, err := minio.New("localhost:9000", &minio.Options{
        Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
        Secure: false,
    })
    if err != nil {
        log.Fatalf("Failed to initialize MinIO client: %v", err)
    }

    // Create bucket if it doesn’t exist
    err = client.MakeBucket(ctx, *bucket, minio.MakeBucketOptions{})
    if err != nil {
        // Check if bucket already exists
        if minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
            log.Fatalf("Failed to create bucket %s: %v", *bucket, err)
        }
    }

    // Upload the file
    _, err = client.FPutObject(ctx, *bucket, *file, *file, minio.PutObjectOptions{})
    if err != nil {
        log.Fatalf("Failed to upload %s to bucket %s: %v", *file, *bucket, err)
    }

    log.Printf("Successfully uploaded %s to bucket %s", *file, *bucket)
}
