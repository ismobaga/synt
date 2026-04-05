package s3util

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMinIOClientUploadDownloadDelete(t *testing.T) {
	t.Helper()

	var uploadedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/synt-assets/folder/file.txt":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			uploadedBody = string(body)
			w.Header().Set("ETag", `"abc123"`)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodHead && r.URL.Path == "/synt-assets/folder/file.txt":
			w.Header().Set("Content-Length", "5")
			w.Header().Set("ETag", `"abc123"`)
			w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/synt-assets/folder/file.txt":
			w.Header().Set("Content-Length", "5")
			w.Header().Set("ETag", `"abc123"`)
			w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
			_, _ = w.Write([]byte("hello"))
		case r.Method == http.MethodDelete && r.URL.Path == "/synt-assets/folder/file.txt":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client, err := NewMinIOClient(Config{
		Endpoint:         server.URL,
		AccessKey:        "minioadmin",
		SecretKey:        "minioadmin",
		Bucket:           "synt-assets",
		AutoCreateBucket: false,
	})
	if err != nil {
		t.Fatalf("NewMinIOClient returned error: %v", err)
	}

	url, err := client.Upload(context.Background(), "/folder/file.txt", strings.NewReader("hello"), "text/plain")
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if !strings.Contains(uploadedBody, "hello") {
		t.Fatalf("expected uploaded body to contain payload, got %q", uploadedBody)
	}
	if url != server.URL+"/synt-assets/folder/file.txt" {
		t.Fatalf("unexpected object URL: %s", url)
	}

	rc, err := client.Download(context.Background(), "folder/file.txt")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected downloaded body: %q", string(data))
	}

	if err := client.Delete(context.Background(), "folder/file.txt"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestNewFromEnvUsesMinIO(t *testing.T) {
	t.Setenv("STORAGE_PROVIDER", "minio")
	t.Setenv("MINIO_ENDPOINT", "http://localhost:9000")
	t.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	t.Setenv("MINIO_SECRET_KEY", "minioadmin")
	t.Setenv("MINIO_BUCKET", "synt")
	t.Setenv("MINIO_PUBLIC_BASE_URL", "http://localhost:9000")

	client, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv returned error: %v", err)
	}

	minioClient, ok := client.(*MinIOClient)
	if !ok {
		t.Fatalf("expected *MinIOClient, got %T", client)
	}
	if minioClient.bucket != "synt" {
		t.Fatalf("unexpected bucket: %s", minioClient.bucket)
	}
	if minioClient.URL("video.mp4") != "http://localhost:9000/synt/video.mp4" {
		t.Fatalf("unexpected URL: %s", minioClient.URL("video.mp4"))
	}
}
