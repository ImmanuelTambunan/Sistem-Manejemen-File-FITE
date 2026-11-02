package service

import (
	"archive/zip"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"fite-arsip-api/internal/apperr"
)

var mimeByExt = map[string]string{
	"pdf":  "application/pdf",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"zip":  "application/zip",
	"png":  "image/png",
	"jpg":  "image/jpeg",
}

var officeDir = map[string]string{"docx": "word/", "xlsx": "xl/", "pptx": "ppt/"}

var blockedInner = map[string]bool{
	"exe": true, "php": true, "phtml": true, "js": true, "sh": true, "bat": true, "cmd": true,
	"com": true, "scr": true, "msi": true, "jar": true, "vbs": true, "ps1": true, "html": true, "htm": true, "svg": true,
}

const formatMsg = "Format berkas harus salah satu dari: pdf, docx, xlsx, pptx, zip, png, jpg. " +
	"Video atau berkas besar tidak diunggah; tulis tautannya pada berkas teks lalu kompres menjadi .zip."

type UploadInfo struct {
	OriginalName string
	Ext          string
	Mime         string
	SizeBytes    int64
	File         multipart.File
}

// InspectUpload memeriksa ukuran, ekstensi, ekstensi ganda, dan isi berkas. Pemanggil menutup File.
func InspectUpload(fh *multipart.FileHeader, maxBytes int64) (*UploadInfo, error) {
	if fh.Size > maxBytes {
		return nil, apperr.FieldError("file", "Ukuran berkas maksimal 10 MB.")
	}
	if fh.Size == 0 {
		return nil, apperr.FieldError("file", "Berkas kosong.")
	}

	name := SanitizeName(fh.Filename)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "jpeg" {
		ext = "jpg"
	}
	if _, ok := mimeByExt[ext]; !ok {
		return nil, apperr.FieldError("file", formatMsg)
	}

	parts := strings.Split(strings.ToLower(name), ".")
	if len(parts) > 2 {
		for _, p := range parts[1 : len(parts)-1] {
			if blockedInner[p] {
				return nil, apperr.FieldError("file", "Nama berkas mengandung ekstensi ganda yang tidak diizinkan.")
			}
		}
	}

	f, err := fh.Open()
	if err != nil {
		return nil, err
	}

	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		_ = f.Close()
		return nil, err
	}

	if err := verifyContent(ext, head[:n], f, fh.Size); err != nil {
		_ = f.Close()
		return nil, err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}

	return &UploadInfo{
		OriginalName: name,
		Ext:          ext,
		Mime:         mimeByExt[ext],
		SizeBytes:    fh.Size,
		File:         f,
	}, nil
}

func verifyContent(ext string, head []byte, f io.ReaderAt, size int64) error {
	bad := apperr.FieldError("file", "Isi berkas tidak sesuai dengan ekstensinya (."+ext+").")
	sniff := http.DetectContentType(head)

	switch ext {
	case "pdf":
		if !bytes.HasPrefix(head, []byte("%PDF-")) {
			return bad
		}
	case "png":
		if sniff != "image/png" {
			return bad
		}
	case "jpg":
		if sniff != "image/jpeg" {
			return bad
		}
	case "zip", "docx", "xlsx", "pptx":
		if !bytes.HasPrefix(head, []byte("PK\x03\x04")) {
			return bad
		}
		zr, err := zip.NewReader(f, size)
		if err != nil {
			return bad
		}
		if dir, office := officeDir[ext]; office {
			var hasTypes, hasDir bool
			for i, zf := range zr.File {
				if i > 10000 {
					break
				}
				hasTypes = hasTypes || zf.Name == "[Content_Types].xml"
				hasDir = hasDir || strings.HasPrefix(zf.Name, dir)
			}
			if !hasTypes || !hasDir {
				return bad
			}
		}
	}
	return nil
}

// SanitizeName menghilangkan jalur direktori dan karakter berbahaya; batas 200 karakter.
func SanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsControl(r):
			continue
		case unicode.IsLetter(r), unicode.IsDigit(r), r == ' ', r == '.', r == '_', r == '-', r == '(', r == ')':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.TrimSpace(b.String())
	if rs := []rune(out); len(rs) > 200 {
		out = string(rs[len(rs)-200:]) // pertahankan ekstensi di ujung
	}
	if out == "" || out == "." || out == ".." {
		return "berkas"
	}
	return out
}
