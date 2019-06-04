package gotenberg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

const (
	resultFilename string = "resultFilename"
	waitTimeout    string = "waitTimeout"
	webhookURL     string = "webhookURL"
)

// Client facilitates interacting with
// the Gotenberg API.
type Client struct {
	Hostname string
}

// Request is a type for sending
// form values and form files to
// the Gotenberg API.
type Request interface {
	postURL() string
	formValues() map[string]string
	formFiles() map[string]string
}

type request struct {
	values map[string]string
}

func newRequest() *request {
	return &request{
		values: make(map[string]string),
	}
}

// ResultFilename sets resultFilename form field.
func (req *request) ResultFilename(filename string) {
	req.values[resultFilename] = filename
}

// WaitTiemout sets waitTimeout form field.
func (req *request) WaitTimeout(timeout float64) {
	req.values[waitTimeout] = strconv.FormatFloat(timeout, 'f', 2, 64)
}

// WebhookURL sets webhookURL form field.
func (req *request) WebhookURL(url string) {
	req.values[webhookURL] = url
}

func (req *request) formValues() map[string]string {
	return req.values
}

// Post sends a request to the Gotenberg API
// and returns the response.
func (c *Client) Post(req Request, body *bytes.Buffer) (*http.Response, error) {
	body, contentType, err := multipartForm(req, body)
	if err != nil {
		return nil, err
	}
	URL := fmt.Sprintf("%s%s", c.Hostname, req.postURL())
	resp, err := http.Post(URL, contentType, body) /* #nosec */
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Store creates the resulting PDF to given destination.
func (c *Client) Store(req Request, dest string) error {
	if hasWebhook(req) {
		return errors.New("cannot use Store method with a webhook")
	}
	resp, err := c.Post(req, nil)
	if err != nil {
		return err
	}
	return writeNewFile(dest, resp.Body)
}

func hasWebhook(req Request) bool {
	webhookURL, ok := req.formValues()[webhookURL]
	if !ok {
		return false
	}
	return webhookURL != ""
}

func writeNewFile(fpath string, in io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}
	out, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("%s: creating new file: %v", fpath, err)
	}
	defer out.Close() // nolint: errcheck
	err = out.Chmod(0644)
	if err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("%s: changing file mode: %v", fpath, err)
	}
	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("%s: writing file: %v", fpath, err)
	}
	return nil
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

func multipartForm(req Request, body *bytes.Buffer) (*bytes.Buffer, string, error) {
	writer := multipart.NewWriter(body)
	defer writer.Close() // nolint: errcheck
	for name, value := range req.formValues() {
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", fmt.Errorf("%s: writing form field: %v", name, err)
		}
	}
	return body, writer.FormDataContentType(), nil
}
