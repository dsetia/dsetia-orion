package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "context"
    "orion/common"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
    _ "github.com/lib/pq"
)

// ObjectInfo holds extracted object metadata
type ObjectInfo struct {
    Type      string
    Filename  string
    Version   string
    TenantID  int64 // Only for "rules"
    Bucket    string
    ObjectPath string
}

// readConfig reads and parses a JSON config file
func readConfig[T any](filename string) (T, error) {
    var config T
    data, err := os.ReadFile(filename)
    if err != nil {
        return config, fmt.Errorf("failed to read config file %s: %w", filename, err)
    }
    if err := json.Unmarshal(data, &config); err != nil {
        return config, fmt.Errorf("failed to parse config file %s: %w", filename, err)
    }
    return config, nil
}

// connectMinio establishes a MinIO client connection
func connectMinio(cfg common.MinioConfig) (*minio.Client, error) {
    client, err := minio.New(cfg.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
        Secure: cfg.UseSSL,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to connect to MinIO: %w", err)
    }
    return client, nil
}

// parseObjectInfo extracts metadata from the filename and validates inputs
func parseObjectInfo(objectType, filename string, tenantID int64) (ObjectInfo, error) {
    info := ObjectInfo{
        Type:     objectType,
        Filename: filename,
    }

    // Validate object type
    switch objectType {
    case "software", "rules", "threatintel":
    default:
        return info, fmt.Errorf("invalid object type: %s; must be 'software', 'rules', or 'threatintel'", objectType)
    }

    // Validate tenant ID for rules
    if objectType == "rules" && tenantID <= 0 {
        return info, fmt.Errorf("tenant ID is required for rules and must be positive")
    }
    if objectType != "rules" && tenantID != 0 {
        return info, fmt.Errorf("tenant ID should only be provided for rules")
    }
    if objectType == "rules" {
        info.TenantID = tenantID
    }

    // Regular expressions for filename formats
    swRegex := regexp.MustCompile(`^hndr-sw-(v\d+\.\d+\.\d+)\.tar\.gz$`)
    rulesRegex := regexp.MustCompile(`^hndr-rules-(r\d+\.\d+\.\d+)\.tar\.gz$`)
    threatRegex := regexp.MustCompile(`^threatintel-(\d{4}\.\d{2}\.\d{2}\.\d{4})\.tar\.gz$`)

    base := filepath.Base(filename)
    switch objectType {
    case "software":
        if !swRegex.MatchString(base) {
            return info, fmt.Errorf("invalid software filename format: %s; expected hndr-sw-vX.Y.Z.tar.gz", base)
        }
        info.Version = swRegex.FindStringSubmatch(base)[1]
        info.Bucket = "software"
        info.ObjectPath = fmt.Sprintf("/hndr-sw-%s.tar.gz", info.Version)

    case "rules":
        if !rulesRegex.MatchString(base) {
            return info, fmt.Errorf("invalid rules filename format: %s; expected hndr-rules-rX.Y.Z.tar.gz", base)
        }
        info.Version = rulesRegex.FindStringSubmatch(base)[1]
        info.Bucket = "rules"
        info.ObjectPath = fmt.Sprintf("/%d/hndr-rules-%s.tar.gz", tenantID, info.Version)

    case "threatintel":
        if !threatRegex.MatchString(base) {
            return info, fmt.Errorf("invalid threatintel filename format: %s; expected threatintel-YYYY.MM.DD.HHMM.tar.gz", base)
        }
        info.Version = threatRegex.FindStringSubmatch(base)[1]
        info.Bucket = "threatintel"
        info.ObjectPath = fmt.Sprintf("/threatintel-%s.tar.gz", info.Version)
    }

    // Normalize object path (remove leading slash for MinIO)
    info.ObjectPath = strings.TrimPrefix(info.ObjectPath, "/")
    return info, nil
}

// computeSHA256 calculates the SHA256 hash of a file
func computeSHA256(filename string) (string, int64, error) {
    file, err := os.Open(filename)
    if err != nil {
        return "", 0, fmt.Errorf("failed to open file %s: %w", filename, err)
    }
    defer file.Close()

    hash := sha256.New()
    size, err := io.Copy(hash, file)
    if err != nil {
        return "", 0, fmt.Errorf("failed to compute SHA256 for %s: %w", filename, err)
    }

    hashSum := hash.Sum(nil)
    return hex.EncodeToString(hashSum), size, nil
}

// uploadToMinio uploads the file to MinIO
func uploadToMinio(client *minio.Client, info ObjectInfo, filename string) error {
    _, err := client.FPutObject(context.Background(), info.Bucket, info.ObjectPath, filename, minio.PutObjectOptions{})
    if err != nil {
        return fmt.Errorf("failed to upload %s to MinIO bucket %s at path %s: %w", filename, info.Bucket, info.ObjectPath, err)
    }
    return nil
}

// updateDatabase inserts the object metadata into the appropriate table
func (db *DB) updateDatabase(info ObjectInfo, size int64, sha256 string) error {
    switch info.Type {
    case "software":
	id, err := db.InsertHndrSw(info.Version, size, sha256)
        if err != nil {
            return fmt.Errorf("failed to insert into hndr_sw: %w", err)
        }
	if id > 0 {
	    fmt.Printf("HndrSw inserted: ID=%d\n", id)
        }

    case "rules":
	id, err := db.InsertHndrRules(info.TenantID, info.Version, size, sha256)
        if err != nil {
            return fmt.Errorf("failed to insert into hndr_rules: %w", err)
        }
	if id > 0 {
	    fmt.Printf("HndrRules inserted: ID=%d\n", id)
        }

    case "threatintel":
	id, err := db.InsertThreatIntel(info.Version, size, sha256)
        if err != nil {
            return fmt.Errorf("failed to insert into threatintel: %w", err)
        }
	if id > 0 {
	    fmt.Printf("ThreatIntel inserted: ID=%d\n", id)
        }
    }
    return nil
}

func main() {
    // Define command-line flags
    dbConfigFile := flag.String("dbconfig", "", "Path to PostgreSQL config file")
    minioConfigFile := flag.String("minioconfig", "", "Path to MinIO config file")
    objectType := flag.String("type", "", "Object type (software, rules, threatintel)")
    tenantID := flag.Int64("tenantid", 0, "Tenant ID (required for rules)")
    filename := flag.String("file", "", "Path to the file to upload")
    flag.Parse()

    // Validate required flags
    if *dbConfigFile == "" || *minioConfigFile == "" || *objectType == "" || *filename == "" {
        fmt.Println("Usage: uploader --dbconfig <db-config.json> --minioconfig <minio-config.json> --type <software|rules|threatintel> --file <file.tar.gz> [--tenantid <id>]")
        flag.PrintDefaults()
        os.Exit(1)
    }

    // Read configuration files
    dbConfig, err := readConfig[common.DBConfig](*dbConfigFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    minioConfig, err := readConfig[common.MinioConfig](*minioConfigFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Parse and validate object info
    info, err := parseObjectInfo(*objectType, *filename, *tenantID)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Connect to PostgreSQL
    db, err := NewDB(dbConfig.ConnString())
    if err != nil {
        fmt.Errorf("failed to connect to database: %w", err)
	os.Exit(1)
    }
    if err := db.Ping(); err != nil {
        fmt.Errorf("failed to ping database: %w", err)
	os.Exit(1)
    }
    defer db.Close()

    // Validate tenant ID for rules
    if info.Type == "rules" {
        _, err := db.ValidateTenant(info.TenantID)
	if err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    }

    // Connect to MinIO
    minioClient, err := connectMinio(minioConfig)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Compute SHA256 and file size
    sha256, size, err := computeSHA256(*filename)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Upload to MinIO
    if err := uploadToMinio(minioClient, info, *filename); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("Successfully uploaded %s to MinIO bucket %s at path %s\n", *filename, info.Bucket, info.ObjectPath)

    // Update database
    if err := db.updateDatabase(info, size, sha256); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("Successfully updated database table %s with version %s\n", info.Type, info.Version)
}
