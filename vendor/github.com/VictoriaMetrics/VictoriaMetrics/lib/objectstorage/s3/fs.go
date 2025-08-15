package s3

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	libfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	credsFile = flag.String("storage.s3.credsFile", "", "Path to file with S3 credentials. Credentials are loaded from default locations if not set.\n"+
		"See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	configFile = flag.String("storage.s3.configFile", "", "Path to file with S3 configs. Configs are loaded from default location if not set.\n"+
		"See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	profileName = flag.String("storage.s3.profileName", "", "Profile name for S3 configs.\n"+
		"If no set, the value of the environment variable will be loaded (AWS_PROFILE or AWS_DEFAULT_PROFILE), or if both not set, DefaultSharedConfigProfile is used")
	forcePathStyle     = flag.Bool("storage.s3.forcePathStyle", true, "Prefixing endpoint with bucket name when set false, true by default.")
	endpoint           = flag.String("storage.s3.endpoint", "", "Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set")
	region             = flag.String("storage.s3.region", "us-east-1", "AWS region for S3 bucket")
	insecureSkipVerify = flag.Bool("storage.s3.insecureSkipVerify", false, "Whether to skip TLS verification when connecting to the S3 endpoint.")
)

type FS struct {
	ctx    context.Context
	bucket string
	prefix string
	client *s3.Client
}

func New(bucket, prefix string) (*FS, error) {
	ctx := context.TODO()
	configOpts := []func(*config.LoadOptions) error{
		config.WithDefaultRegion(*region),
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.Backoff = retry.NewExponentialJitterBackoff(3 * time.Minute)
				o.MaxAttempts = 10
				o.Retryables = append(retry.DefaultRetryables, retry.RetryableErrorCode{
					Codes: map[string]struct{}{
						"IncompleteBody": {},
						"ExpiredToken":   {},
					},
				})
			})
		}),
	}
	if len(*profileName) > 0 {
		configOpts = append(configOpts, config.WithSharedConfigProfile(*profileName))
	}
	if len(*configFile) > 0 {
		configOpts = append(configOpts, config.WithSharedConfigFiles([]string{
			*configFile,
		}))
	}
	if len(*credsFile) > 0 {
		configOpts = append(configOpts, config.WithSharedCredentialsFiles([]string{
			*credsFile,
		}))
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		configOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot load S3 config: %w", err)
	}

	tr := httputil.NewTransport(true, "objectstorage_s3_client")
	if *insecureSkipVerify {
		tr.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	cfg.HTTPClient = &http.Client{
		Transport: tr,
	}
	var outerErr error
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if len(*endpoint) > 0 {
			logger.Infof("Using provided custom S3 endpoint: %s", *endpoint)
			o.UsePathStyle = *forcePathStyle
			o.BaseEndpoint = endpoint
		} else {
			r, err := manager.GetBucketRegion(ctx, s3.NewFromConfig(cfg), bucket)
			if err != nil {
				outerErr = fmt.Errorf("cannot determine region for bucket %q: %w", bucket, err)
				return
			}

			o.Region = r
			logger.Infof("bucket %q is stored at region %q; switching to this region", bucket, region)
		}
	})
	if outerErr != nil {
		return nil, outerErr
	}
	fs := &FS{
		client: client,
		bucket: bucket,
		prefix: prefix,
		ctx:    ctx,
	}
	return fs, nil
}

func (*FS) ReadFile(path string) ([]byte, error) {
	return nil, nil
}

func (*FS) MustFileSize(path string) uint64 {
	return 0
}

func (fs *FS) MustHardLinkFiles(srcDir, dstDir string) {}

func (fs *FS) GetPath(p string) string {
	return "s3://" + filepath.Join(fs.bucket, fs.prefix, p)
}

func (*FS) MustMkdirFailIfExist(_ string) {}

func (*FS) MustGetFreeSpace() uint64 {
	return uint64(1<<64 - 1)
}

func (*FS) MustGetTotalSpace() uint64 {
	return uint64(1<<64 - 1)
}

func (*FS) MustClose(f *os.File) {}

func (*FS) MustMkdirIfNotExist(_ string) {}

func (*FS) IsPathExist(_ string) bool {
	return true
}

func (*FS) MustSyncPathAndParentDir(_ string) {}

func (*FS) MustReadDir(dir string) []os.DirEntry {
	return nil
}

func (*FS) MustCreateFlockFile() *os.File {
	return nil
}

func (*FS) IsPartiallyRemovedDir(dir string) bool {
	return false
}

func (*FS) MustRemoveDir(dir string) {}

func (*FS) MustWriteAtomic(_ string, _ []byte, _ bool) {}

func (*FS) IsDirOrSymlink(de os.DirEntry) bool {
	return false
}

func (*FS) MustSyncPath(_ string) {}

func (*FS) MustWriteSync(path string, data []byte) {}

func (*FS) MustOpenReaderAt(path string) *libfs.ReaderAt {
	return nil
}

func (fs *FS) Upload(p string) error {
	opts := transfermanager.Options{}
	tm := transfermanager.New(fs.client, opts)
	input := transfermanager.UploadDirectoryInput{
		Bucket:    fs.bucket,
		Source:    fs.prefix,
		Recursive: true,
		KeyPrefix: fs.prefix,
	}
	_, err := tm.UploadDirectory(fs.ctx, &input)
	if err != nil {
		return fmt.Errorf("failed to upload %s to s3://%s/%s: %w", p, fs.bucket, fs.prefix, err)
	}
	return nil
}
