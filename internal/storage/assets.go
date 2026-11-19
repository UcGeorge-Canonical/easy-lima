package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/UcGeorge-Canonical/easy-lima/internal/host/files"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Asset struct {
	URL           string `json:"url"`
	SHA256        string `json:"sha256"`
	MaxBytes      int64  `json:"max_bytes"`
	ExpandedBound int64  `json:"expanded_bound"`
}

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func AssetPath(root string, a Asset) string {
	return filepath.Join(root, "_easy-lima", "assets", "sha256", a.SHA256)
}
func VerifyAsset(path string, a Asset) error {
	i, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !i.Mode().IsRegular() || i.Size() > a.MaxBytes {
		return model.Fail("config.drift", "The image asset has an unexpected type or size.")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, io.LimitReader(f, a.MaxBytes+1)); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return model.Fail("config.drift", "The image content does not match its pinned SHA-256.")
	}
	return nil
}
func Materialize(ctx context.Context, root, opID string, a Asset) (string, error) {
	if !digestPattern.MatchString(a.SHA256) || a.MaxBytes <= 0 || a.MaxBytes > 32<<30 {
		return "", model.Invalid("asset", "a SHA-256 and bounded allocation are required")
	}
	dest := AssetPath(root, a)
	if err := files.PrivateDir(filepath.Dir(dest)); err != nil {
		return "", err
	}
	g, err := files.Lock(ctx, dest+".lock", true, true)
	if err != nil {
		return "", err
	}
	defer g.Close()
	if _, err = os.Lstat(dest); err == nil {
		return dest, VerifyAsset(dest, a)
	}
	tmpdir := filepath.Join(root, "_easy-lima", "tmp", opID)
	if err = files.PrivateDir(tmpdir); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(tmpdir, "image-*.partial")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	u, err := url.Parse(a.URL)
	if err != nil {
		return "", err
	}
	var source io.ReadCloser
	if u.Scheme == "https" {
		client := &http.Client{Timeout: 30 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 || req.URL.Scheme != "https" {
				return model.Invalid("asset URL", "unsafe redirect")
			}
			return nil
		}}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", model.Fail("operation.failed", "The pinned image could not be downloaded.")
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", model.Fail("operation.failed", "The pinned image publisher returned an unsuccessful response.")
		}
		if resp.ContentLength > a.MaxBytes {
			resp.Body.Close()
			return "", model.Fail("storage.insufficient", "The image exceeds its admitted download bound.")
		}
		source = resp.Body
	} else if u.Scheme == "file" {
		source, err = os.Open(u.Path)
		if err != nil {
			return "", err
		}
	} else {
		return "", model.Invalid("asset URL", "only pinned HTTPS or local file assets are accepted")
	}
	defer source.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(source, a.MaxBytes+1))
	if err != nil {
		return "", model.Fail("operation.partial", "The pinned image download was interrupted; the VM has not been recreated.")
	}
	if n > a.MaxBytes || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return "", model.Fail("config.drift", "Downloaded image size or digest does not match the frozen input.")
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return dest, files.SyncDir(filepath.Dir(dest))
}
