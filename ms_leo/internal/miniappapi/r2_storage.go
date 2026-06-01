package miniappapi

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// R2Storage — обёртка над Cloudflare R2 (S3-совместимый API) для фото тренировок.
// Если в конфиге не заданы все поля, клиент не создаётся и используется локальный диск.
type R2Storage struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string // без / на конце
}

// NewR2Storage возвращает (nil, nil), если R2 не сконфигурирован (это не ошибка — просто fallback на диск).
func NewR2Storage(accountID, accessKeyID, secretAccessKey, bucket, publicBaseURL string) (*R2Storage, error) {
	accountID = strings.TrimSpace(accountID)
	accessKeyID = strings.TrimSpace(accessKeyID)
	secretAccessKey = strings.TrimSpace(secretAccessKey)
	bucket = strings.TrimSpace(bucket)
	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucket == "" || publicBaseURL == "" {
		return nil, nil
	}
	endpoint := accountID + ".r2.cloudflarestorage.com"
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: true,
		Region: "auto",
	})
	if err != nil {
		return nil, fmt.Errorf("r2 client init: %w", err)
	}
	return &R2Storage{client: client, bucket: bucket, publicBaseURL: publicBaseURL}, nil
}

// Upload кладёт объект в бакет и возвращает его публичный URL.
func (r *R2Storage) Upload(ctx context.Context, objectName, contentType string, data []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := r.client.PutObject(ctx, r.bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
		// Фото иммутабельны (имя = случайный токен), можно кэшировать надолго в Telegram WebView/CDN.
		CacheControl: "public, max-age=31536000, immutable",
	})
	if err != nil {
		return "", fmt.Errorf("r2 put %q: %w", objectName, err)
	}
	return r.publicBaseURL + "/" + objectName, nil
}

// Delete удаляет объект из бакета. objectName — это базовое имя файла (как в публичном URL).
func (r *R2Storage) Delete(ctx context.Context, objectName string) error {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := r.client.RemoveObject(ctx, r.bucket, objectName, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("r2 delete %q: %w", objectName, err)
	}
	return nil
}

// PublicBaseURL — публичный адрес бакета (для распознавания собственных ссылок при удалении).
func (r *R2Storage) PublicBaseURL() string {
	if r == nil {
		return ""
	}
	return r.publicBaseURL
}

// objectNameFromURL возвращает имя объекта, если URL принадлежит этому бакету, иначе "".
// Так мы не пытаемся удалять чужие/легаси-ссылки (например, локальный /api/miniapp/media/...).
func (r *R2Storage) objectNameFromURL(rawURL string) string {
	if r == nil {
		return ""
	}
	rawURL = strings.TrimSpace(rawURL)
	prefix := r.publicBaseURL + "/"
	if rawURL == "" || !strings.HasPrefix(rawURL, prefix) {
		return ""
	}
	name := strings.TrimPrefix(rawURL, prefix)
	// Имя — это случайный токен с расширением, без вложенных путей.
	if name == "" || strings.ContainsAny(name, "/?#") {
		return ""
	}
	return name
}

// DeleteByURL удаляет объект по его публичному URL (best-effort: чужие ссылки игнорирует).
func (r *R2Storage) DeleteByURL(ctx context.Context, rawURL string) error {
	name := r.objectNameFromURL(rawURL)
	if name == "" {
		return nil
	}
	return r.Delete(ctx, name)
}
