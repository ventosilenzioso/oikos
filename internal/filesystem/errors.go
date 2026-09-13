package filesystem

import "errors"

var (
	ErrPathOutsideRoot = errors.New("path berada di luar root server")
	ErrNotFound        = errors.New("file tidak ditemukan")
	ErrPermission      = errors.New("permission ditolak")
	ErrServerNotFound  = errors.New("server tidak ditemukan")
	ErrQuotaExceeded   = errors.New("quota disk terlampaui")
)
