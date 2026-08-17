package mdmhub

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEnrollmentProfileHandler(t *testing.T) {
	cfg, err := makeEnrollConfig("https://mdm.example.com", "com.apple.mgmt.External.test", false, "chal", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := EnrollmentProfileHandler(cfg)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/enroll/profile", strings.NewReader(`<?xml version="1.0"?><plist></plist>`))
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", method, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, want := range []string{
			"application/x-apple-aspen-config",
			"https://mdm.example.com/mdm",
			"https://mdm.example.com/scep",
			"com.apple.mgmt.External.test",
			"chal",
		} {
			if want == "application/x-apple-aspen-config" {
				if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, want) {
					t.Fatalf("%s content-type=%s", method, ct)
				}
				continue
			}
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q", method, want)
			}
		}
	}
}

func TestEnrollmentProfileIncludesCA(t *testing.T) {
	cfg, err := makeEnrollConfig("https://mdm.example.com", "com.apple.mgmt.External.test", false, "", []byte("not-a-cert"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.caDER) != 0 {
		t.Fatal("expected invalid CA PEM to be ignored")
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	cfg, err = makeEnrollConfig("https://mdm.example.com", "com.apple.mgmt.External.test", false, "chal", caPEM)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	EnrollmentProfileHandler(cfg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/enroll/profile", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "com.apple.security.root") {
		t.Fatal("missing CA payload")
	}
	if !strings.Contains(body, "CAFingerprint") {
		t.Fatal("missing CAFingerprint")
	}
}

func TestMakeEnrollConfigRequiresURL(t *testing.T) {
	if _, err := makeEnrollConfig("", "topic", false, "", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestMountProtocolAcceptsADEMethods(t *testing.T) {
	cfg, err := makeEnrollConfig("https://mdm.example.com", "com.apple.mgmt.External.test", false, "chal", nil)
	if err != nil {
		t.Fatal(err)
	}
	noop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	MountProtocol(mux, &Hub{
		Version: noop,
		MDM:     noop,
		SCEP:    noop,
		Enroll:  EnrollmentProfileHandler(cfg),
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/enroll/profile", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s /enroll/profile status=%d (ADE would fail to download the profile)", method, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/x-apple-aspen-config") {
			t.Fatalf("%s content-type=%s", method, ct)
		}
	}
}
