package mdmhub

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/micromdm/plist"
)

type enrollConfig struct {
	publicURL     string
	mdmTopic      string
	checkin       bool
	scepURL       string
	scepChallenge string
	caDER         []byte
}

func makeEnrollConfig(publicURL, mdmTopic string, checkin bool, scepChallenge string, caPEM []byte) (*enrollConfig, error) {
	if publicURL == "" {
		return nil, errors.New("MDM_PUBLIC_URL is required for /enroll")
	}
	u, err := url.Parse(publicURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, errors.New("MDM_PUBLIC_URL must include scheme and host")
	}
	publicURL = strings.TrimRight(publicURL, "/")
	if mdmTopic == "" {
		return nil, errors.New("MDM_TOPIC is required for /enroll")
	}
	var caDER []byte
	if block, _ := pem.Decode(caPEM); block != nil && block.Type == "CERTIFICATE" {
		if _, err := x509.ParseCertificate(block.Bytes); err == nil {
			caDER = block.Bytes
		}
	}
	return &enrollConfig{
		publicURL:     publicURL,
		mdmTopic:      mdmTopic,
		checkin:       checkin,
		scepURL:       publicURL + "/scep",
		scepChallenge: scepChallenge,
		caDER:         caDER,
	}, nil
}

// EnrollmentProfileHandler serves an unsigned enrollment .mobileconfig.
// ADE devices GET, POST, or PUT this URL during Setup Assistant.
func EnrollmentProfileHandler(cfg *enrollConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("enrollment profile requested",
			"method", r.Method,
			"path", r.URL.Path,
			"ua", r.UserAgent(),
			"remote", r.RemoteAddr,
		)
		if cfg == nil {
			http.Error(w, "enrollment profile not configured", http.StatusInternalServerError)
			return
		}
		profileUUID := uuid.NewString()
		scepUUID := uuid.NewString()
		mdmUUID := uuid.NewString()

		serverURL := cfg.publicURL + "/mdm"
		checkInURL := serverURL
		if cfg.checkin {
			checkInURL = cfg.publicURL + "/checkin"
		}

		// Keep SCEP keys minimal. Extra Subject/KeyUsage fields have caused
		// ADE "profile installation failed" on some iOS versions.
		scepPayloadContent := map[string]any{
			"URL":     cfg.scepURL,
			"Name":    "Device Identity",
			"Keysize": 2048,
		}
		if cfg.scepChallenge != "" {
			scepPayloadContent["Challenge"] = cfg.scepChallenge
		}
		if len(cfg.caDER) > 0 {
			sum := sha1.Sum(cfg.caDER)
			scepPayloadContent["CAFingerprint"] = sum[:]
		}

		payloads := make([]any, 0, 3)
		if len(cfg.caDER) > 0 {
			payloads = append(payloads, map[string]any{
				"PayloadType":                "com.apple.security.root",
				"PayloadVersion":             1,
				"PayloadIdentifier":          "com.schoolmdm.enroll.ca",
				"PayloadUUID":                uuid.NewString(),
				"PayloadDisplayName":         "School MDM CA",
				"PayloadDescription":         "Trusts the MDM SCEP certificate authority",
				"PayloadOrganization":        "SchoolMDM",
				"PayloadCertificateFileName": "scep-ca.cer",
				"PayloadContent":             cfg.caDER,
			})
		}
		payloads = append(payloads,
			map[string]any{
				"PayloadType":         "com.apple.security.scep",
				"PayloadVersion":      1,
				"PayloadIdentifier":   "com.schoolmdm.enroll.scep",
				"PayloadUUID":         scepUUID,
				"PayloadDisplayName":  "School MDM SCEP",
				"PayloadDescription":  "Configures SCEP for device identity certificate",
				"PayloadOrganization": "SchoolMDM",
				"PayloadContent":      scepPayloadContent,
			},
			map[string]any{
				"PayloadType":             "com.apple.mdm",
				"PayloadVersion":          1,
				"PayloadIdentifier":       "com.schoolmdm.enroll.mdm",
				"PayloadUUID":             mdmUUID,
				"PayloadDisplayName":      "School MDM",
				"PayloadDescription":      "Enrolls device with MDM server",
				"PayloadOrganization":     "SchoolMDM",
				"AccessRights":            8191,
				"CheckInURL":              checkInURL,
				"ServerURL":               serverURL,
				"CheckOutWhenRemoved":     true,
				"IdentityCertificateUUID": scepUUID,
				"Topic":                   cfg.mdmTopic,
				"SignMessage":             true,
				"UseDevelopmentAPNS":      false,
				"ServerCapabilities":      []string{"com.apple.mdm.per-user-connections"},
			},
		)

		profile := map[string]any{
			"PayloadDisplayName":       "School MDM Enrollment",
			"PayloadDescription":       "Enrolls this device with School MDM",
			"PayloadIdentifier":        "com.schoolmdm.enroll",
			"PayloadOrganization":      "SchoolMDM",
			"PayloadType":              "Configuration",
			"PayloadUUID":              profileUUID,
			"PayloadVersion":           1,
			"PayloadScope":             "System",
			"PayloadRemovalDisallowed": false,
			"PayloadContent":           payloads,
		}

		raw, err := plist.Marshal(profile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-apple-aspen-config")
		w.Header().Set("Content-Disposition", `attachment; filename="enroll.mobileconfig"`)
		_, _ = w.Write(raw)
	})
}
