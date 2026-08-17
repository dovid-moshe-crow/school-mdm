// Package mdmhub mounts Apple MDM protocol handlers (/mdm, /scep, /enroll, /version).
package mdmhub

import (
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/dwdmsh/school-mdm/internal/mdmscep"
	"github.com/micromdm/nanolib/log"
	"github.com/micromdm/nanolib/log/stdlogfmt"
	nanoapi "github.com/micromdm/nanomdm/api"
	"github.com/micromdm/nanomdm/certverify"
	"github.com/micromdm/nanomdm/cryptoutil"
	nanohttpmdm "github.com/micromdm/nanomdm/http/mdm"
	"github.com/micromdm/nanomdm/push/nanopush"
	pushservice "github.com/micromdm/nanomdm/push/service"
	"github.com/micromdm/nanomdm/service"
	"github.com/micromdm/nanomdm/service/certauth"
	"github.com/micromdm/nanomdm/service/multi"
	"github.com/micromdm/nanomdm/service/nanomdm"
	nanostorage "github.com/micromdm/nanomdm/storage"
	"github.com/micromdm/nanomdm/storage/pgsql"
)

// Config configures the protocol plane.
type Config struct {
	PublicURL     string
	MDMTopic      string
	Checkin       bool
	CertHeader    string // if set, extract device cert from this header; else Mdm-Signature
	SCEPPass      string
	SCEPChallenge string
	SCEPCN        string
	SCEPOrg       string
	Debug         bool
	// OnTokenUpdate optionally runs after each TokenUpdate (enroll / token refresh).
	OnTokenUpdate OnTokenUpdateFunc
}

// Hub holds mounted protocol handlers and the push enqueuer.
type Hub struct {
	MDM       http.Handler
	Checkin   http.Handler // nil unless Checkin enabled
	SCEP      http.Handler
	Enroll    http.Handler
	Version   http.Handler
	PushEnq   *nanoapi.PushEnqueuer
	Store     nanostorage.ServiceStore
	CAPEM     []byte
	cfg       Config
}

// New builds protocol services against an sql.DB with search_path=mdm.
func New(db *sql.DB, cfg Config) (*Hub, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}
	if cfg.SCEPPass == "" {
		return nil, fmt.Errorf("SCEP CA password required")
	}
	logger := stdlogfmt.New(stdlogfmt.WithDebugFlag(cfg.Debug))

	nanoStore, err := pgsql.New(pgsql.WithDB(db), pgsql.WithLogger(logger.With("storage", "pgsql")))
	if err != nil {
		return nil, fmt.Errorf("nanomdm storage: %w", err)
	}

	scepCfg := mdmscep.DefaultConfig()
	scepCfg.CAPass = cfg.SCEPPass
	scepCfg.StaticChallenge = cfg.SCEPChallenge
	if cfg.SCEPCN != "" {
		scepCfg.CACN = cfg.SCEPCN
	}
	if cfg.SCEPOrg != "" {
		scepCfg.CAOrg = cfg.SCEPOrg
	}
	scepSvc, err := mdmscep.NewService(db, scepCfg, logger.With("service", "scep"))
	if err != nil {
		return nil, fmt.Errorf("scep: %w", err)
	}

	// Verify SCEP CA PEM parses (fail fast).
	if err := validateCAPEM(scepSvc.CAPEM()); err != nil {
		return nil, err
	}

	pushSvc := pushservice.New(nanoStore, nanoStore, nanopush.NewFactory(), logger.With("service", "push"))
	pushEnq, err := nanoapi.NewPushEnqueuer(nanoStore, pushSvc, nanoapi.WithLogger(logger.With("service", "enqueue")))
	if err != nil {
		return nil, fmt.Errorf("push enqueuer: %w", err)
	}

	var svc service.CheckinAndCommandService = nanomdm.New(
		nanoStore,
		nanomdm.WithLogger(logger.With("service", "nanomdm")),
		nanomdm.WithUserAuthenticate(nanomdm.NewUAService(nanoStore, false)),
	)
	svc = certauth.New(svc, nanoStore, certauth.WithLogger(logger.With("service", "certauth")))
	if cfg.OnTokenUpdate != nil {
		svc = multi.New(
			logger.With("service", "multi"),
			svc,
			&tokenUpdateOnboard{fn: cfg.OnTokenUpdate},
		)
	}

	verifier, err := certverify.NewPoolVerifier(scepSvc.CAPEM(), nil, x509.ExtKeyUsageClientAuth)
	if err != nil {
		return nil, fmt.Errorf("cert verifier: %w", err)
	}

	mdmHandler := http.Handler(nanohttpmdm.CheckinAndCommandHandler(svc, logger.With("handler", "mdm")))
	var checkinHandler http.Handler
	if cfg.Checkin {
		checkinHandler = nanohttpmdm.CheckinHandler(svc, logger.With("handler", "checkin"))
		mdmHandler = nanohttpmdm.CommandAndReportResultsHandler(svc, logger.With("handler", "mdm-cmd"))
	}
	mdmHandler = wrapAuth(mdmHandler, cfg, verifier, logger)
	if checkinHandler != nil {
		checkinHandler = wrapAuth(checkinHandler, cfg, verifier, logger)
	}

	enrollCfg, err := makeEnrollConfig(cfg.PublicURL, cfg.MDMTopic, cfg.Checkin, cfg.SCEPChallenge, scepSvc.CAPEM())
	var enroll http.Handler
	if err == nil {
		enroll = EnrollmentProfileHandler(enrollCfg)
	} else {
		enroll = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "enrollment not configured: "+err.Error(), http.StatusServiceUnavailable)
		})
	}

	return &Hub{
		MDM:     logProtocol("mdm", mdmHandler),
		Checkin: checkinHandler,
		SCEP:    logProtocol("scep", rewriteSCEPPath(scepSvc.Handler())),
		Enroll:  enroll,
		Version: versionHandler(),
		PushEnq: pushEnq,
		Store:   nanoStore,
		CAPEM:   scepSvc.CAPEM(),
		cfg:     cfg,
	}, nil
}

func wrapAuth(h http.Handler, cfg Config, verifier certverify.CertVerifier, logger log.Logger) http.Handler {
	h = nanohttpmdm.CertVerifyMiddleware(h, verifier, logger.With("mw", "cert-verify"))
	if cfg.CertHeader != "" {
		return nanohttpmdm.CertExtractPEMHeaderMiddleware(h, cfg.CertHeader, logger.With("mw", "cert-header"))
	}
	return nanohttpmdm.CertExtractMdmSignatureMiddleware(
		h,
		nanohttpmdm.MdmSignatureVerifierFunc(cryptoutil.VerifyMdmSignature),
		nanohttpmdm.SigLogWithLogger(logger.With("mw", "mdm-signature")),
		nanohttpmdm.SigLogWithLogErrors(cfg.Debug),
	)
}

func validateCAPEM(pemBytes []byte) error {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return fmt.Errorf("scep CA PEM empty")
	}
	_, err := x509.ParseCertificate(block.Bytes)
	return err
}

// MountProtocol registers protocol routes on mux. Safe to call with nil hub (no-op).
func MountProtocol(mux *http.ServeMux, hub *Hub) {
	if mux == nil || hub == nil {
		return
	}
	mux.Handle("GET /version", hub.Version)
	mux.Handle("/mdm", hub.MDM)
	mux.Handle("/scep", hub.SCEP)
	mux.Handle("/scep/", hub.SCEP)
	// ADE Setup Assistant PUTs/POSTs a device-info plist to the DEP profile URL.
	// Method-restricted GET made those requests 404 (SPA miss) and the phone
	// reported that it could not download a configuration from the MDM server.
	mux.Handle("/enroll", hub.Enroll)
	mux.Handle("/enroll/profile", hub.Enroll)
	if hub.Checkin != nil {
		mux.Handle("/checkin", hub.Checkin)
	}
}

func versionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"school-mdm","mdm":true}`))
	})
}
