// SPDX-License-Identifier: GPL-3.0-or-later
package smtp2go

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coletivo420/smtp2go-helper/internal/message"
	"github.com/coletivo420/smtp2go-helper/internal/sysexits"
	sdk "github.com/smtp2go-oss/smtp2go-go"
)

func TestSendUsesSDKShapeOnlyEnvelopeAndFastacceptBool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/email/send" || r.Method != "POST" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Smtp2go-Api-Key") != "api-01234567890123456789012345678901" {
			t.Error("API header missing")
		}
		var v map[string]any
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			t.Error(err)
		}
		if v["fastaccept"] != false {
			t.Errorf("fastaccept not boolean false: %#v", v["fastaccept"])
		}
		to := v["to"].([]any)
		if len(to) != 1 || to[0] != "b@example.com" {
			t.Errorf("to=%#v", to)
		}
		if len(v["cc"].([]any)) != 0 || len(v["bcc"].([]any)) != 0 {
			t.Errorf("unexpected recipient lists: cc=%#v bcc=%#v", v["cc"], v["bcc"])
		}
		if v["sender"] != "a@example.com" {
			t.Errorf("sender=%v", v["sender"])
		}
		att := v["attachments"].([]any)[0].(map[string]any)
		if att["filename"] != "one.txt" || att["fileblob"] != "b25l" || att["mimetype"] != "text/plain" {
			t.Errorf("attachment=%#v", att)
		}
		inline := v["inlines"].([]any)[0].(map[string]any)
		if inline["filename"] != "pic" || inline["fileblob"] != "cG5n" {
			t.Errorf("inline=%#v", inline)
		}
		headers := v["custom_headers"].([]any)[0].(map[string]any)
		if headers["header"] != "Reply-To" {
			t.Errorf("headers=%#v", headers)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"request_id":"req-1","data":{"succeeded":1,"failed":0,"failures":[],"email_id":"mail-1"}}`))
	}))
	defer server.Close()
	c := New(server.URL+"/v3/email/send", time.Second, "api-01234567890123456789012345678901", false)
	m := message.Message{From: "a@example.com", To: "b@example.com", Subject: "s", TextBody: "x",
		Attachments:   []message.Attachment{{Filename: "one.txt", MIMEType: "text/plain", Bytes: []byte("one")}},
		Inlines:       []message.Inline{{ContentID: "pic", Filename: "pic.png", MIMEType: "image/png", Bytes: []byte("png")}},
		CustomHeaders: []message.Header{{Name: "Reply-To", Value: "reply@example.com"}},
	}
	got, err := c.Send(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if got.EmailID != "mail-1" || got.Succeeded != 1 {
		t.Fatalf("result=%+v", got)
	}
}

func TestPermissionsEndpointParsesDataArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/api_keys/permissions" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"request_id":"r","data":["/email/send","/api_keys/permissions"]}`))
	}))
	defer server.Close()
	c := New(server.URL+"/v3/email/send", time.Second, "api-01234567890123456789012345678901", false)
	p, err := c.Permissions(context.Background())
	if err != nil || p.Send != "allowed" {
		t.Fatalf("permissions=%+v err=%v", p, err)
	}
}

func TestHTTPClientTimeoutIsTemporary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{"succeeded":1,"failed":0}`))
	}))
	defer server.Close()
	c := New(server.URL+"/v3/email/send", 10*time.Millisecond, "api-01234567890123456789012345678901", false)
	_, err := c.Send(context.Background(), message.Message{From: "a@example.com", To: "b@example.com", Subject: "test", TextBody: "body"})
	ae, ok := err.(*APIError)
	if !ok || ae.ExitCode != sysexits.TEMPFAIL {
		t.Fatalf("timeout error=%#v", err)
	}
}

func TestInvalidEnvelopeAndSubjectNeverCallHTTP(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{"succeeded":1}`)) }))
	defer server.Close()
	c := New(server.URL+"/v3/email/send", time.Second, "api-01234567890123456789012345678901", false)
	for _, m := range []message.Message{{From: "a@example.com", To: "a@example.com,b@example.com", Subject: "x", TextBody: "body"}, {From: "a@example.com", To: "b@example.com", Subject: "bad\nSubject", TextBody: "body"}} {
		_, err := c.Send(context.Background(), m)
		ae, ok := err.(*APIError)
		if !ok || ae.ExitCode != sysexits.DATA {
			t.Fatalf("error=%#v", err)
		}
	}
	if calls != 0 {
		t.Fatalf("request count=%d", calls)
	}
}

func TestClassifyResponses(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   int
	}{{"success", 200, `{"succeeded":1,"failed":0}`, 0}, {"result success", 200, `{"result":"success"}`, 0}, {"empty success", 200, `{}`, sysexits.TEMPFAIL}, {"failed unclassified", 200, `{"succeeded":0,"failed":1,"failures":[{"error":"bad"}]}`, sysexits.TEMPFAIL}, {"partial unclassified", 200, `{"succeeded":1,"failed":1,"failures":["bad"]}`, sysexits.TEMPFAIL}, {"failed permanent", 200, `{"succeeded":0,"failed":1,"failures":[{"error":"invalid recipient address"}]}`, sysexits.DATA}, {"400 endpoint permission", 400, `{"error":"no permission","error_code":"E_ApiResponseCodes.ENDPOINT_PERMISSION_DENIED"}`, sysexits.TEMPFAIL}, {"400 message rejection", 400, `{"error":"invalid recipient","failures":[{"to":"bad"}]}`, sysexits.DATA}, {"401", 401, `{"error":"denied"}`, sysexits.TEMPFAIL}, {"403", 403, `{}`, sysexits.TEMPFAIL}, {"408", 408, `{}`, sysexits.TEMPFAIL}, {"425", 425, `{}`, sysexits.TEMPFAIL}, {"429", 429, `{}`, sysexits.TEMPFAIL}, {"500", 500, `{}`, sysexits.TEMPFAIL}, {"503", 503, `{}`, sysexits.TEMPFAIL}, {"invalid JSON", 200, `no`, sysexits.TEMPFAIL}, {"empty JSON", 200, ``, sysexits.TEMPFAIL}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evaluate(tc.status, []byte(tc.body), nil)
			if tc.want == 0 && err != nil {
				t.Fatal(err)
			}
			if tc.want != 0 {
				ae, ok := err.(*APIError)
				if !ok || ae.ExitCode != tc.want {
					t.Fatalf("err=%#v want exit %d", err, tc.want)
				}
			}
		})
	}
}

func TestUpstreamModelJSONContract(t *testing.T) {
	b, err := json.Marshal(sdk.Email{From: "f", To: []string{"t"}, Attachments: []*sdk.EmailBinaryData{{Filename: "a", Fileblob: "YQ==", MimeType: "text/plain"}}, CustomHeaders: []*sdk.EmailCustomHeader{{Header: "Reply-To", Value: "x"}}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"sender", "to", "attachments", "custom_headers"} {
		if _, ok := got[k]; !ok {
			t.Errorf("upstream JSON field %q missing", k)
		}
	}
	a := got["attachments"].([]any)[0].(map[string]any)
	for _, k := range []string{"filename", "fileblob", "mimetype"} {
		if _, ok := a[k]; !ok {
			t.Errorf("attachment field %q missing", k)
		}
	}
}
