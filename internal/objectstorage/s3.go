// Package objectstorage implements the narrow signed private object boundary.
// It deliberately has no bucket administration, deletion, public URL or SDK
// default retry. An ambiguous create-only upload requires custody reconciliation.
package objectstorage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const MaxObjectBytes int64 = 256 * 1024 * 1024

var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
var regionPattern = regexp.MustCompile(`^[a-z]{2}-[a-z0-9-]+-[0-9]+$`)
var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_/-]+(?:\.[a-zA-Z0-9.]+)?$`)
var shaPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Config struct {
	Mode         string `json:"mode"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	Endpoint     string `json:"endpoint,omitempty"`
	AccessKeyID  string `json:"access_key_id"`
	SecretKey    string `json:"secret_access_key"`
	SessionToken string `json:"session_token,omitempty"`
}

type Client struct {
	config Config
	base   *url.URL
	http   *http.Client
	signer *v4.Signer
}

type Acknowledgement struct {
	Version string
	SHA256  string
	Bytes   int64
}

type UploadError struct {
	Code      string
	Uncertain bool
}

func (e *UploadError) Error() string { return "private object upload was not verified: " + e.Code }

func ReadPrivate(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > limit || info.Size() <= 0 {
		return nil, errors.New("input must be an available bounded private regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("private input is unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("private input changed while opening")
	}
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, errors.New("private input exceeds its limit")
	}
	return body, nil
}

func DecodePrivate(path string, into any) error {
	body, err := ReadPrivate(path, 32*1024)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(into); err != nil {
		return errors.New("private configuration is invalid or has unsupported fields")
	}
	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("private configuration contains additional data")
	}
	return nil
}

func New(config Config, fixtureAllowed bool) (*Client, error) {
	if !regionPattern.MatchString(config.Region) || !bucketPattern.MatchString(config.Bucket) || len(config.Prefix) > 120 || !keyPattern.MatchString(config.Prefix) || strings.Contains(config.Prefix, "..") || strings.HasPrefix(config.Prefix, "/") || strings.HasSuffix(config.Prefix, "/") || config.AccessKeyID == "" || len(config.AccessKeyID) > 128 || config.SecretKey == "" || len(config.SecretKey) > 256 || len(config.SessionToken) > 4096 {
		return nil, errors.New("private storage identity, prefix or signing credentials are invalid")
	}
	endpoint := "https://" + config.Bucket + ".s3." + config.Region + ".amazonaws.com"
	if config.Mode == "FIXTURE" && fixtureAllowed {
		endpoint = config.Endpoint
	} else if config.Mode != "S3" || config.Endpoint != "" {
		return nil, errors.New("storage requires an explicit fixture or regional private S3 destination")
	}
	base, err := url.Parse(endpoint)
	if err != nil || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return nil, errors.New("private storage endpoint is invalid")
	}
	if config.Mode == "FIXTURE" {
		ip := net.ParseIP(base.Hostname())
		if base.Scheme != "http" || ip == nil || !ip.IsLoopback() || base.Port() == "" {
			return nil, errors.New("fixture storage requires an explicit numeric-loopback HTTP address")
		}
	} else if base.Scheme != "https" || base.Hostname() != config.Bucket+".s3."+config.Region+".amazonaws.com" {
		return nil, errors.New("storage endpoint must match the bound HTTPS S3 destination")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Transport: transport, Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{config: config, base: base, http: client, signer: v4.NewSigner()}, nil
}

func (c *Client) request(ctx context.Context, method, key, version string, body io.Reader, size int64, hash string, now time.Time) (*http.Request, error) {
	if len(key) > 300 || !keyPattern.MatchString(key) || strings.Contains(key, "..") || !strings.HasPrefix(key, c.config.Prefix+"/") {
		return nil, errors.New("object identity is outside the bound private prefix")
	}
	u := *c.base
	u.Path = "/" + key
	if version != "" {
		if version == "null" || len(version) > 1024 || strings.ContainsAny(version, "\r\n") {
			return nil, errors.New("an exact non-null object version is required")
		}
		u.RawQuery = url.Values{"versionId": {version}}.Encode()
	}
	r, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	r.ContentLength = size
	r.Header.Set("X-Amz-Content-Sha256", hash)
	if method == http.MethodPut {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return nil, errors.New("object checksum is invalid")
		}
		r.Header.Set("If-None-Match", "*")
		r.Header.Set("Content-Type", "application/octet-stream")
		r.Header.Set("X-Amz-Checksum-Sha256", base64.StdEncoding.EncodeToString(decoded))
	} else {
		r.Header.Set("X-Amz-Checksum-Mode", "ENABLED")
	}
	credentials := aws.Credentials{AccessKeyID: c.config.AccessKeyID, SecretAccessKey: c.config.SecretKey, SessionToken: c.config.SessionToken}
	err = c.signer.SignHTTP(ctx, credentials, r, hash, "s3", c.config.Region, now.UTC(), func(options *v4.SignerOptions) { options.DisableURIPathEscaping = true })
	return r, err
}

func (c *Client) Put(ctx context.Context, key, path, expectedHash string, expectedBytes int64, now time.Time) (Acknowledgement, error) {
	var ack Acknowledgement
	if !shaPattern.MatchString(expectedHash) || expectedBytes <= 0 || expectedBytes > MaxObjectBytes {
		return ack, &UploadError{Code: "UPLOAD_REJECTED"}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() != expectedBytes {
		return ack, &UploadError{Code: "UPLOAD_REJECTED"}
	}
	f, err := os.Open(path)
	if err != nil {
		return ack, &UploadError{Code: "UPLOAD_REJECTED"}
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return ack, &UploadError{Code: "UPLOAD_REJECTED"}
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, MaxObjectBytes+1))
	if err != nil || n != expectedBytes || hex.EncodeToString(h.Sum(nil)) != expectedHash {
		return ack, &UploadError{Code: "UPLOAD_REJECTED"}
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return ack, &UploadError{Code: "UPLOAD_REJECTED"}
	}
	r, err := c.request(ctx, http.MethodPut, key, "", f, expectedBytes, expectedHash, now)
	if err != nil {
		return ack, &UploadError{Code: "UPLOAD_REJECTED"}
	}
	response, err := c.http.Do(r)
	if err != nil {
		return ack, &UploadError{Code: "UPLOAD_UNCERTAIN", Uncertain: true}
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ack, &UploadError{Code: "UPLOAD_DENIED"}
	}
	if response.StatusCode != http.StatusOK {
		uncertain := response.StatusCode == 409 || response.StatusCode == 412 || response.StatusCode >= 500
		code := "UPLOAD_REJECTED"
		if uncertain {
			code = "UPLOAD_UNCERTAIN"
		}
		return ack, &UploadError{Code: code, Uncertain: uncertain}
	}
	checksum, err := base64.StdEncoding.Strict().DecodeString(response.Header.Get("X-Amz-Checksum-Sha256"))
	version := response.Header.Get("X-Amz-Version-Id")
	if err != nil || len(checksum) != sha256.Size || hex.EncodeToString(checksum) != expectedHash || version == "" || version == "null" || len(version) > 1024 {
		return ack, &UploadError{Code: "UPLOAD_UNCERTAIN", Uncertain: true}
	}
	return Acknowledgement{Version: version, SHA256: expectedHash, Bytes: expectedBytes}, nil
}

// Custody retrieval has a separate configured principal. The live upload
// worker never calls this and requires no GetObject or decryption capability.
func (c *Client) GetVersion(ctx context.Context, key, version, destination, expectedHash string, expectedBytes int64, now time.Time) error {
	if version == "" || !shaPattern.MatchString(expectedHash) || expectedBytes <= 0 || expectedBytes > MaxObjectBytes {
		return errors.New("independent object identity, version, size and checksum are required")
	}
	empty := sha256.Sum256(nil)
	r, err := c.request(ctx, http.MethodGet, key, version, nil, 0, hex.EncodeToString(empty[:]), now)
	if err != nil {
		return err
	}
	response, err := c.http.Do(r)
	if err != nil {
		return errors.New("exact custody retrieval is unavailable")
	}
	defer response.Body.Close()
	checksum, err := base64.StdEncoding.Strict().DecodeString(response.Header.Get("X-Amz-Checksum-Sha256"))
	if response.StatusCode != 200 || response.Header.Get("X-Amz-Version-Id") != version || err != nil || hex.EncodeToString(checksum) != expectedHash || response.ContentLength != expectedBytes {
		return errors.New("retrieved version acknowledgement differs from the retained original")
	}
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		f.Close()
		if !complete {
			os.Remove(destination)
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(response.Body, expectedBytes+1))
	if err != nil || n != expectedBytes || hex.EncodeToString(h.Sum(nil)) != expectedHash {
		return errors.New("retrieved encrypted bytes differ from the retained original")
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}
