// Package mdmdep mounts NanoDEP under /dep (same surface as nanok).
package mdmdep

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/micromdm/nanolib/log/stdlogfmt"
	depclient "github.com/micromdm/nanodep/client"
	dephttp "github.com/micromdm/nanodep/http"
	depapi "github.com/micromdm/nanodep/http/api"
	depapinext "github.com/micromdm/nanodep/http/apinext"
	depproxy "github.com/micromdm/nanodep/proxy"
	"github.com/micromdm/nanodep/storage"
	depstorepgsql "github.com/micromdm/nanodep/storage/pgsql"
)

// Mount builds the /dep mux (caller strips /dep and applies auth except /dep/version).
func Mount(db *sql.DB, debug bool) (handler http.Handler, store storage.AllStorage, err error) {
	if db == nil {
		return nil, nil, fmt.Errorf("nil db")
	}
	depStore, err := depstorepgsql.New(depstorepgsql.WithDB(db))
	if err != nil {
		return nil, nil, fmt.Errorf("nanodep storage: %w", err)
	}
	logger := stdlogfmt.New(stdlogfmt.WithDebugFlag(debug)).With("service", "nanodep")
	depMux := http.NewServeMux()
	depMux.Handle("/version", dephttp.VersionHandler("school-mdm"))

	handleStripped := func(h http.Handler, endpoint string) {
		depMux.Handle(endpoint, http.StripPrefix(endpoint, h))
	}

	tokensMux := dephttp.NewMethodMux()
	tokensMux.Handle("PUT", depapi.StoreAuthTokensHandler(depStore, logger.With("handler", "store-auth-tokens")))
	tokensMux.Handle("GET", depapi.RetrieveAuthTokensHandler(depStore, logger.With("handler", "retrieve-auth-tokens")))
	handleStripped(tokensMux, "/v1/tokens/")

	configMux := dephttp.NewMethodMux()
	configMux.Handle("GET", depapi.RetrieveConfigHandler(depStore, logger.With("handler", "retrieve-config")))
	configMux.Handle("PUT", depapi.StoreConfigHandler(depStore, logger.With("handler", "store-config")))
	handleStripped(configMux, "/v1/config/")

	tokenPKIMux := dephttp.NewMethodMux()
	tokenPKIMux.Handle("GET", depapi.GetCertTokenPKIHandler(depStore, logger.With("handler", "get-token-pki")))
	tokenPKIMux.Handle("PUT", depapi.DecryptTokenPKIHandler(depStore, depStore, logger.With("handler", "put-token-pki")))
	handleStripped(tokenPKIMux, "/v1/tokenpki/")

	assignerMux := dephttp.NewMethodMux()
	assignerMux.Handle("GET", depapi.RetrieveAssignerProfileHandler(depStore, logger.With("handler", "retrieve-assigner-profile")))
	assignerMux.Handle("PUT", depapi.StoreAssignerProfileHandler(depStore, logger.With("handler", "store-assigner-profile")))
	handleStripped(assignerMux, "/v1/assigner/")

	namesMux := dephttp.NewMethodMux()
	namesMux.Handle("GET", depapinext.NewQueryDEPNamesHandler(depStore, logger.With("handler", "query-dep-names")))
	handleStripped(namesMux, "/v1/dep_names")

	handleStripped(depapi.NewBypassCodeHandler(), "/v1/bypasscode")
	handleStripped(depapi.NewMAIDJWTHandler(depStore, logger.With("handler", "get-maid-jwt"), uuid.NewString), "/v1/maidjwt/")

	p := depproxy.New(
		depclient.NewTransport(http.DefaultTransport, http.DefaultClient, depStore, nil),
		depStore,
		logger.With("component", "proxy"),
	)
	var baseProxy http.Handler = depproxy.ProxyDEPNameHandler(p, logger.With("handler", "proxy"))
	baseProxy = http.StripPrefix("/proxy/", baseProxy)
	depMux.Handle("/proxy/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("Authorization")
		baseProxy.ServeHTTP(w, r)
	}))

	return http.StripPrefix("/dep", depMux), depStore, nil
}
