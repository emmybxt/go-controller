package gocontroller

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// FileHeader represents an uploaded file.
type FileHeader struct {
	Name     string
	Header   *multipart.FileHeader
	OpenFunc func() (multipart.File, error)
}

// Open opens the uploaded file.
func (f *FileHeader) Open() (multipart.File, error) {
	return f.OpenFunc()
}

// BindFile parses a single file upload from a multipart/form-data request.
func (c *Context) BindFile(field string) (*FileHeader, error) {
	if c.maxBodyBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.ResponseWriter, c.Request.Body, c.maxBodyBytes)
	}

	err := c.Request.ParseMultipartForm(c.maxBodyBytes)
	if err != nil {
		return nil, &APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "invalid_file_upload",
			Message:    "failed to parse multipart form",
			Cause:      err,
		}
	}

	file, header, err := c.Request.FormFile(field)
	if err != nil {
		return nil, &APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "missing_file",
			Message:    fmt.Sprintf("file field %q not found", field),
			Cause:      err,
		}
	}

	return &FileHeader{
		Name:   header.Filename,
		Header: header,
		OpenFunc: func() (multipart.File, error) {
			return file, nil
		},
	}, nil
}

// BindFiles parses multiple file uploads from a multipart/form-data request.
func (c *Context) BindFiles(field string) ([]*FileHeader, error) {
	if c.maxBodyBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.ResponseWriter, c.Request.Body, c.maxBodyBytes)
	}

	err := c.Request.ParseMultipartForm(c.maxBodyBytes)
	if err != nil {
		return nil, &APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "invalid_file_upload",
			Message:    "failed to parse multipart form",
			Cause:      err,
		}
	}

	form := c.Request.MultipartForm
	if form == nil || form.File[field] == nil {
		return nil, &APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "missing_file",
			Message:    fmt.Sprintf("file field %q not found", field),
			Cause:      nil,
		}
	}

	headers := form.File[field]
	files := make([]*FileHeader, len(headers))
	for i, h := range headers {
		files[i] = &FileHeader{
			Name:   h.Filename,
			Header: h,
			OpenFunc: func() (multipart.File, error) {
				return h.Open()
			},
		}
	}

	return files, nil
}

// BindForm parses both files and form values from a multipart/form-data request.
func (c *Context) BindForm() (map[string][]string, []*FileHeader, error) {
	if c.maxBodyBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.ResponseWriter, c.Request.Body, c.maxBodyBytes)
	}

	err := c.Request.ParseMultipartForm(c.maxBodyBytes)
	if err != nil {
		return nil, nil, &APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "invalid_form",
			Message:    "failed to parse multipart form",
			Cause:      err,
		}
	}

	form := c.Request.MultipartForm
	if form == nil {
		return nil, nil, nil
	}

	var files []*FileHeader
	for field, headers := range form.File {
		for _, h := range headers {
			files = append(files, &FileHeader{
				Name:   h.Filename,
				Header: h,
				OpenFunc: func() (multipart.File, error) {
					return h.Open()
				},
			})
		}
		_ = field
	}

	return form.Value, files, nil
}

// SaveFile saves an uploaded file to the specified directory.
func SaveFile(destDir string, file *FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	filename := filepath.Base(file.Name)
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		return "", fmt.Errorf("invalid file name")
	}
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create destination directory: %w", err)
	}

	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("copy file contents: %w", err)
	}

	return destPath, nil
}
