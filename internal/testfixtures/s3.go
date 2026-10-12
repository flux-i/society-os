// Package testfixtures provides independent numeric-loopback protocol peers.
// Application packages never import this package at runtime.
package testfixtures

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"society.local/portal/internal/objectstorage"
)

type StoredObject struct {
	Key, Version, SHA256 string
	Bytes                []byte
}

type S3 struct {
	Server       *httptest.Server
	mu           sync.Mutex
	mode         string
	objects      map[string]StoredObject
	writerSecret string
	readerSecret string
	puts, gets   int
}

func randomSecret() string {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(key[:])
}

func NewS3(t *testing.T) *S3 {
	t.Helper()
	s := &S3{objects: map[string]StoredObject{}, writerSecret: randomSecret(), readerSecret: randomSecret()}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Server.Close)
	return s
}

func (s *S3) Config(role string) objectstorage.Config {
	secret := s.writerSecret
	if role == "reader" {
		secret = s.readerSecret
	}
	return objectstorage.Config{Mode: "FIXTURE", Region: "ap-south-1", Bucket: "fictional-recovery", Prefix: "recovery/society", Endpoint: s.Server.URL, AccessKeyID: "fixture-" + role, SecretKey: secret}
}

func (s *S3) SetMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
}

func (s *S3) Counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.puts, s.gets
}

func (s *S3) Objects() []StoredObject {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []StoredObject
	for _, object := range s.objects {
		object.Bytes = append([]byte(nil), object.Bytes...)
		result = append(result, object)
	}
	return result
}

func keyed(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	return h.Sum(nil)
}

// Independent SigV4 verification: no application signer or AWS SDK is used.
func verifySignature(r *http.Request, body []byte, expectedID, secret string) bool {
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "AWS4-HMAC-SHA256 ") {
		return false
	}
	fields := map[string]string{}
	for _, field := range strings.Split(strings.TrimPrefix(authorization, "AWS4-HMAC-SHA256 "), ", ") {
		name, value, ok := strings.Cut(field, "=")
		if !ok {
			return false
		}
		fields[name] = value
	}
	credential := strings.Split(fields["Credential"], "/")
	if len(credential) != 5 || credential[0] != expectedID || credential[2] != "ap-south-1" || credential[3] != "s3" || credential[4] != "aws4_request" {
		return false
	}
	date := r.Header.Get("X-Amz-Date")
	if len(date) != 16 || date[:8] != credential[1] {
		return false
	}
	payload := sha256.Sum256(body)
	if r.Header.Get("X-Amz-Content-Sha256") != hex.EncodeToString(payload[:]) {
		return false
	}
	var headers strings.Builder
	for _, name := range strings.Split(fields["SignedHeaders"], ";") {
		value := r.Header.Get(name)
		if name == "host" {
			value = r.Host
		} else if name == "content-length" {
			value = strconv.FormatInt(r.ContentLength, 10)
		}
		headers.WriteString(name + ":" + strings.Join(strings.Fields(value), " ") + "\n")
	}
	canonical := r.Method + "\n" + r.URL.EscapedPath() + "\n" + strings.ReplaceAll(r.URL.Query().Encode(), "+", "%20") + "\n" + headers.String() + "\n" + fields["SignedHeaders"] + "\n" + hex.EncodeToString(payload[:])
	canonicalHash := sha256.Sum256([]byte(canonical))
	scope := strings.Join(credential[1:], "/")
	toSign := "AWS4-HMAC-SHA256\n" + date + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])
	k := keyed([]byte("AWS4"+secret), credential[1])
	k = keyed(k, credential[2])
	k = keyed(k, "s3")
	k = keyed(k, "aws4_request")
	return hmac.Equal([]byte(fields["Signature"]), []byte(hex.EncodeToString(keyed(k, toSign))))
}

func (s *S3) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8*1024*1024+1))
	if err != nil || len(body) > 8*1024*1024 {
		w.WriteHeader(400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	role, secret := "fixture-writer", s.writerSecret
	if r.Method == "GET" {
		role, secret = "fixture-reader", s.readerSecret
	}
	if !verifySignature(r, body, role, secret) || !strings.HasPrefix(r.URL.Path, "/recovery/society/") {
		w.WriteHeader(403)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/")
	if r.Method == "PUT" {
		s.puts++
		if r.Header.Get("If-None-Match") != "*" || r.URL.RawQuery != "" {
			w.WriteHeader(400)
			return
		}
		if s.mode == "DENIED" {
			w.WriteHeader(403)
			return
		}
		if s.mode == "REJECTED" {
			w.WriteHeader(400)
			return
		}
		if s.mode == "SERVICE_ERROR" {
			w.WriteHeader(503)
			return
		}
		if s.mode == "REDIRECT" {
			w.Header().Set("Location", s.Server.URL+"/redirect-target")
			w.WriteHeader(302)
			return
		}
		checksum := sha256.Sum256(body)
		if r.Header.Get("X-Amz-Checksum-Sha256") != base64.StdEncoding.EncodeToString(checksum[:]) {
			w.WriteHeader(400)
			return
		}
		if _, exists := s.objects[key]; exists {
			w.WriteHeader(412)
			return
		}
		object := StoredObject{Key: key, Version: fmt.Sprintf("fixture-version-%d", len(s.objects)+1), SHA256: hex.EncodeToString(checksum[:]), Bytes: append([]byte(nil), body...)}
		s.objects[key] = object
		if s.mode == "LOST_REPLY" {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				connection.Close()
			}
			return
		}
		if s.mode != "MISSING_CHECKSUM" {
			value := base64.StdEncoding.EncodeToString(checksum[:])
			if s.mode == "WRONG_CHECKSUM" {
				value = base64.StdEncoding.EncodeToString(make([]byte, 32))
			}
			w.Header().Set("X-Amz-Checksum-Sha256", value)
		}
		if s.mode != "MISSING_VERSION" {
			value := object.Version
			if s.mode == "NULL_VERSION" {
				value = "null"
			}
			w.Header().Set("X-Amz-Version-Id", value)
		}
		w.WriteHeader(200)
		return
	}
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	s.gets++
	object, found := s.objects[key]
	if !found || r.URL.Query().Get("versionId") != object.Version || r.Header.Get("X-Amz-Checksum-Mode") != "ENABLED" {
		w.WriteHeader(404)
		return
	}
	decoded, _ := hex.DecodeString(object.SHA256)
	w.Header().Set("X-Amz-Version-Id", object.Version)
	w.Header().Set("X-Amz-Checksum-Sha256", base64.StdEncoding.EncodeToString(decoded))
	bytes := object.Bytes
	if s.mode == "CORRUPT_GET" {
		bytes = append([]byte(nil), bytes...)
		bytes[0] ^= 1
	}
	if s.mode == "WRONG_GET_VERSION" {
		w.Header().Set("X-Amz-Version-Id", "another-version")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	w.WriteHeader(200)
	w.Write(bytes)
}
