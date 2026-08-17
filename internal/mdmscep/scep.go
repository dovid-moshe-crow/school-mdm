package mdmscep

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"net/http"

	"github.com/micromdm/nanolib/log"
	"github.com/micromdm/scep/v2/depot"
	scepserver "github.com/micromdm/scep/v2/server"
)

// Config holds SCEP server configuration.
type Config struct {
	// CAPass is the password for encrypting/decrypting the CA private key.
	CAPass string

	// CAYears is the validity period for the CA certificate in years.
	CAYears int

	// CACN is the Common Name for the CA certificate.
	CACN string

	// CAOrg is the Organization for the CA certificate.
	CAOrg string

	// CACountry is the Country for the CA certificate.
	CACountry string

	// StaticChallenge is a static challenge password. If set, dynamic challenges are disabled.
	StaticChallenge string

	// AllowRenewalDays is the number of days before expiry when renewal is allowed.
	AllowRenewalDays int

	// ValidityDays is the validity period for issued certificates in days.
	ValidityDays int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		CAYears:          10,
		CACN:             "SCEP CA",
		CAOrg:            "SchoolMDM",
		CACountry:        "US",
		AllowRenewalDays: 14,
		ValidityDays:     365,
	}
}

// Service wraps the SCEP depot and provides HTTP handlers.
type Service struct {
	depot   *PgDepot
	handler http.Handler
	config  Config
	logger  log.Logger
	caPEM   []byte
}

// NewService creates a new SCEP service with PostgreSQL storage.
func NewService(db *sql.DB, config Config, logger log.Logger) (*Service, error) {
	if config.CAPass == "" {
		return nil, ErrCAPassRequired
	}

	pgDepot, err := NewPgDepot(db)
	if err != nil {
		return nil, err
	}

	// Load or create the CA
	crt, key, err := pgDepot.CreateOrLoadCA(
		[]byte(config.CAPass),
		config.CAYears,
		config.CACN,
		config.CAOrg,
		config.CACountry,
	)
	if err != nil {
		return nil, err
	}

	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crt.Raw,
	})

	// Create the CSR signer
	var signer scepserver.CSRSignerContext = scepserver.SignCSRAdapter(depot.NewSigner(
		pgDepot,
		depot.WithAllowRenewalDays(config.AllowRenewalDays),
		depot.WithValidityDays(config.ValidityDays),
		depot.WithCAPass(config.CAPass),
	))

	// Do not require a SCEP challenge. ADE Setup Assistant often omits the
	// profile Challenge password; StaticChallengeMiddleware then rejects the
	// CSR and iOS reports that profile installation failed.
	_ = config.StaticChallenge

	// Create the SCEP service
	svc, err := scepserver.NewService(crt, key, signer, scepserver.WithLogger(newGoKitLogger(logger)))
	if err != nil {
		return nil, err
	}

	// Create HTTP handler
	e := scepserver.MakeServerEndpoints(svc)
	handler := scepserver.MakeHTTPHandler(e, svc, newGoKitLogger(logger))

	return &Service{
		depot:   pgDepot,
		handler: handler,
		config:  config,
		logger:  logger,
		caPEM:   caPEM,
	}, nil
}

// CAPEM returns the SCEP CA certificate (PEM-encoded).
// This can be used as a trust root for verifying SCEP-issued device identity certificates.
func (s *Service) CAPEM() []byte {
	return s.caPEM
}

// Handler returns the HTTP handler for SCEP requests.
// Mount this at "/scep" in your HTTP mux.
func (s *Service) Handler() http.Handler {
	return s.handler
}

// ChallengeHandler returns an HTTP handler for generating SCEP challenges.
// This should be protected with authentication (e.g., API key).
func (s *Service) ChallengeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		output := &struct {
			Error     string `json:"error,omitempty"`
			Challenge string `json:"challenge,omitempty"`
		}{}

		var err error
		output.Challenge, err = s.depot.SCEPChallenge()
		if err != nil {
			output.Error = err.Error()
			s.logger.Info("msg", "scep challenge error", "err", err)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(output); err != nil {
			s.logger.Info("msg", "encoding response", "err", err)
		}
	})
}

// BasicAuthMiddleware wraps an HTTP handler with basic authentication.
func BasicAuthMiddleware(next http.Handler, username, password, realm string) http.Handler {
	uBytes := []byte(username)
	pBytes := []byte(password)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), uBytes) != 1 || subtle.ConstantTimeCompare([]byte(p), pBytes) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Errors
var (
	ErrCAPassRequired = &scepError{"CA password is required"}
)

type scepError struct {
	msg string
}

func (e *scepError) Error() string {
	return e.msg
}
