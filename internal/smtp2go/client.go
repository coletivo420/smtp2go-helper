// SPDX-License-Identifier: GPL-3.0-or-later
package smtp2go

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coletivo420/smtp2go-helper/internal/envelope"
	"github.com/coletivo420/smtp2go-helper/internal/logsafe"
	"github.com/coletivo420/smtp2go-helper/internal/message"
	"github.com/coletivo420/smtp2go-helper/internal/sysexits"
	sdk "github.com/smtp2go-oss/smtp2go-go"
)

// The upstream SDK supplies the canonical send and attachment structures. Its
// Send() creates an http.Client with no timeout, reads SMTP2GO_API_KEY from the
// environment, and discards HTTP status plus succeeded/failed/failures. We
// therefore use a narrow adapter with the upstream Email model and bounded HTTP.
type Client struct {
	endpoint   string
	timeout    time.Duration
	key        string
	fastAccept bool
	http       *http.Client
}

func New(endpoint string, timeout time.Duration, key string, fastAccept bool) *Client {
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 30 * time.Second
	}
	return &Client{endpoint: endpoint, timeout: timeout, key: key, fastAccept: fastAccept, http: &http.Client{
		Timeout: timeout,
		// The helper treats every redirect as a retryable response. This avoids
		// forwarding the credential or message payload to an untrusted origin.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

type payload struct {
	*sdk.Email
	FastAccept bool `json:"fastaccept"`
}
type Result struct {
	RequestID string
	EmailID   string
	Succeeded int
}
type APIError struct {
	ExitCode int
	Message  string
}

func (e *APIError) Error() string { return e.Message }

func (c *Client) Send(ctx context.Context, m message.Message) (Result, error) {
	to, err := envelope.ParseRecipient(m.To)
	if err != nil {
		return Result{}, &APIError{sysexits.DATA, "invalid message: exactly one valid recipient is required"}
	}
	if m.Subject == "" {
		return Result{}, &APIError{sysexits.DATA, "message has no Subject header; SMTP2GO requires subject"}
	}
	if m.TextBody == "" && m.HTMLBody == "" {
		return Result{}, &APIError{sysexits.DATA, "message has no text/plain or text/html body"}
	}
	if strings.ContainsAny(m.Subject, "\r\n\x00") {
		return Result{}, &APIError{sysexits.DATA, "invalid message Subject header"}
	}
	e := &sdk.Email{
		From: m.From, To: []string{to}, Cc: []string{}, Bcc: []string{},
		Subject: m.Subject, TextBody: m.TextBody, HtmlBody: m.HTMLBody,
		Attachments: []*sdk.EmailBinaryData{}, Inlines: []*sdk.EmailBinaryData{},
		CustomHeaders: []*sdk.EmailCustomHeader{},
	}
	for _, a := range m.Attachments {
		e.Attachments = append(e.Attachments, &sdk.EmailBinaryData{Filename: a.Filename, Fileblob: base64.StdEncoding.EncodeToString(a.Bytes), MimeType: a.MIMEType})
	}
	for _, a := range m.Inlines {
		name := a.ContentID
		if name == "" {
			name = a.Filename
		}
		e.Inlines = append(e.Inlines, &sdk.EmailBinaryData{Filename: name, Fileblob: base64.StdEncoding.EncodeToString(a.Bytes), MimeType: a.MIMEType})
	}
	for _, h := range m.CustomHeaders {
		e.CustomHeaders = append(e.CustomHeaders, &sdk.EmailCustomHeader{Header: h.Name, Value: h.Value})
	}
	b, err := json.Marshal(payload{Email: e, FastAccept: c.fastAccept})
	if err != nil {
		return Result{}, &APIError{sysexits.DATA, "invalid API payload"}
	}
	resp, raw, err := c.request(ctx, c.endpoint, b)
	return evaluate(resp, raw, err)
}

func (c *Client) Permissions(ctx context.Context) (struct{ Send string }, error) {
	endpoint := strings.TrimSuffix(c.endpoint, "/email/send") + "/api_keys/permissions"
	resp, raw, err := c.request(ctx, endpoint, []byte(`{}`))
	if err != nil {
		return struct{ Send string }{}, err
	}
	if resp < 200 || resp >= 300 {
		return struct{ Send string }{}, classifyHTTP(resp, raw)
	}
	var obj struct {
		Data      json.RawMessage `json:"data"`
		Error     string          `json:"error"`
		ErrorCode string          `json:"error_code"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return struct{ Send string }{}, errors.New("invalid permission response")
	}
	if obj.Error != "" || obj.ErrorCode != "" {
		return struct{ Send string }{}, errors.New("permission endpoint returned an API error")
	}
	var endpoints []string
	if json.Unmarshal(obj.Data, &endpoints) != nil {
		return struct{ Send string }{}, errors.New("permission response did not contain endpoint list")
	}
	for _, endpoint := range endpoints {
		if endpoint == "*" || endpoint == "/email/*" || endpoint == "/email/send" {
			return struct{ Send string }{"allowed"}, nil
		}
	}
	return struct{ Send string }{"denied"}, nil
}

func (c *Client) request(ctx context.Context, url string, body []byte) (int, []byte, error) {
	if !authorizedURL(url) {
		return 0, nil, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: refused request to an unauthorized API URL"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, errors.New("request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Smtp2go-Api-Key", c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return 0, nil, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: timeout"}
		}
		return 0, nil, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: temporary API transport failure"}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 {
		return resp.StatusCode, nil, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: invalid or oversized API response"}
	}
	return resp.StatusCode, raw, nil
}

// authorizedURL constrains every authenticated request, including the
// permissions diagnostic, to the documented SMTP2GO API origin and paths.
// This defense remains effective even if a caller bypasses config.Load.
func authorizedURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "api.smtp2go.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return u.Path == "/v3/email/send" || u.Path == "/v3/api_keys/permissions"
}

type apiResponse struct {
	RequestID             string          `json:"request_id"`
	EmailID               string          `json:"email_id"`
	Succeeded             *int            `json:"succeeded"`
	Failed                *int            `json:"failed"`
	Failures              json.RawMessage `json:"failures"`
	Error                 string          `json:"error"`
	ErrorCode             string          `json:"error_code"`
	Result                string          `json:"result"`
	FieldValidationErrors json.RawMessage `json:"field_validation_errors"`
	Data                  struct {
		RequestID             string          `json:"request_id"`
		EmailID               string          `json:"email_id"`
		Succeeded             *int            `json:"succeeded"`
		Failed                *int            `json:"failed"`
		Failures              json.RawMessage `json:"failures"`
		Error                 string          `json:"error"`
		ErrorCode             string          `json:"error_code"`
		Result                string          `json:"result"`
		FieldValidationErrors json.RawMessage `json:"field_validation_errors"`
	} `json:"data"`
}

func evaluate(status int, raw []byte, err error) (Result, error) {
	if err != nil {
		return Result{}, err
	}
	if status == 408 || status == 425 || status == 429 || status >= 500 {
		return Result{}, &APIError{sysexits.TEMPFAIL, fmt.Sprintf("smtp2go-helper: HTTP %d temporary API failure", status)}
	}
	var r apiResponse
	if len(raw) == 0 || json.Unmarshal(raw, &r) != nil {
		return Result{}, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: unclassifiable API response"}
	}
	if r.RequestID == "" {
		r.RequestID = r.Data.RequestID
	}
	if r.EmailID == "" {
		r.EmailID = r.Data.EmailID
	}
	if r.Succeeded == nil {
		r.Succeeded = r.Data.Succeeded
	}
	if r.Failed == nil {
		r.Failed = r.Data.Failed
	}
	if len(r.Failures) == 0 || string(r.Failures) == "null" {
		r.Failures = r.Data.Failures
	}
	if r.Error == "" {
		r.Error = r.Data.Error
	}
	if r.ErrorCode == "" {
		r.ErrorCode = r.Data.ErrorCode
	}
	if r.Result == "" {
		r.Result = r.Data.Result
	}
	if len(r.FieldValidationErrors) == 0 || string(r.FieldValidationErrors) == "null" {
		r.FieldValidationErrors = r.Data.FieldValidationErrors
	}
	if status < 200 || status >= 300 {
		return Result{}, classifyHTTP(status, raw)
	}
	if r.Error != "" || r.ErrorCode != "" || strings.EqualFold(r.Result, "error") {
		if isPermanentMessageFailure(r.ErrorCode+" "+r.Error, r.FieldValidationErrors) {
			return Result{}, &APIError{sysexits.DATA, "smtp2go-helper: message rejected by SMTP2GO"}
		}
		return Result{}, classifyHTTP(status, raw)
	}
	if (r.Failed != nil && *r.Failed > 0) || hasFailures(r.Failures) || hasFailures(r.FieldValidationErrors) {
		if isPermanentMessageFailure(string(r.Failures), r.FieldValidationErrors) {
			return Result{}, &APIError{sysexits.DATA, "smtp2go-helper: message rejected by SMTP2GO"}
		}
		return Result{}, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: SMTP2GO reported an unclassified delivery failure; retained for retry"}
	}
	if r.Succeeded != nil && *r.Succeeded < 1 {
		return Result{}, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: API did not confirm acceptance"}
	}
	if r.Succeeded == nil && r.EmailID == "" {
		if strings.EqualFold(r.Result, "success") {
			v := 1
			r.Succeeded = &v
		} else {
			return Result{}, &APIError{sysexits.TEMPFAIL, "smtp2go-helper: API response did not confirm acceptance"}
		}
	}
	if r.Succeeded == nil {
		v := 1
		r.Succeeded = &v
	}
	return Result{RequestID: logsafe.Token(r.RequestID), EmailID: logsafe.Token(r.EmailID), Succeeded: *r.Succeeded}, nil
}

func classifyHTTP(status int, raw []byte) error {
	if status == 408 || status == 425 || status == 429 || status >= 500 {
		return &APIError{sysexits.TEMPFAIL, fmt.Sprintf("smtp2go-helper: HTTP %d temporary API failure", status)}
	}
	var v apiResponse
	_ = json.Unmarshal(raw, &v)
	code := v.ErrorCode
	if code == "" {
		code = v.Data.ErrorCode
	}
	if code == "E_ApiResponseCodes.ENDPOINT_PERMISSION_DENIED" || status == 401 || status == 403 {
		return &APIError{sysexits.TEMPFAIL, fmt.Sprintf("smtp2go-helper: HTTP %d API authorization/configuration failure", status)}
	}
	if status == 400 && isPermanentMessageFailure(v.ErrorCode+" "+v.Error+" "+string(v.FieldValidationErrors)+" "+string(v.Data.FieldValidationErrors), append(append(v.Failures, v.Data.Failures...), []byte(" ")...)) {
		return &APIError{sysexits.DATA, "smtp2go-helper: message rejected by SMTP2GO (HTTP 400)"}
	}
	if status == 400 {
		return &APIError{sysexits.TEMPFAIL, "smtp2go-helper: HTTP 400 API request rejected; retained for retry"}
	}
	return &APIError{sysexits.TEMPFAIL, fmt.Sprintf("smtp2go-helper: HTTP %d API response requires retry", status)}
}

func isPermanentMessageFailure(details string, validation json.RawMessage) bool {
	text := strings.ToLower(details + " " + string(validation))
	for _, marker := range []string{"permission", "unauthor", "api key", "sender domain", "sender not verified", "temporar", "timeout", "try again", "rate limit"} {
		if strings.Contains(text, marker) {
			return false
		}
	}
	for _, marker := range []string{"invalid recipient", "malformed recipient", "invalid email address", "message too large", "message size", "invalid attachment", "invalid mime", "missing required field", "subject is required"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	if hasFailures(validation) {
		return true
	}
	return false
}
func hasFailures(b json.RawMessage) bool {
	if len(b) == 0 || string(b) == "null" || string(b) == "[]" || string(b) == "{}" {
		return false
	}
	var n int
	if json.Unmarshal(b, &n) == nil {
		return n > 0
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		return s != ""
	}
	var a []any
	if json.Unmarshal(b, &a) == nil {
		return len(a) > 0
	}
	var m map[string]any
	if json.Unmarshal(b, &m) == nil {
		return len(m) > 0
	}
	return true
}
