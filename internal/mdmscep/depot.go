// package mdmscep provides a PostgreSQL-backed SCEP server implementation.
package mdmscep

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/micromdm/scep/v2/depot"
)

// sanitizeForPg sanitizes a string for PostgreSQL text columns by:
// 1. Replacing invalid UTF-8 bytes with the replacement character
// 2. Removing null bytes (0x00) which PostgreSQL doesn't allow in text
func sanitizeForPg(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		// Skip null bytes - PostgreSQL doesn't allow them in text columns
		if r != 0 {
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// PgDepot implements the SCEP depot interface using PostgreSQL.
type PgDepot struct {
	db  *sql.DB
	ctx context.Context
	crt *x509.Certificate
	key *rsa.PrivateKey
}

// NewPgDepot creates a new PostgreSQL-backed SCEP depot.
func NewPgDepot(db *sql.DB) (*PgDepot, error) {
	if db == nil {
		return nil, errors.New("nil database connection")
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return &PgDepot{
		db:  db,
		ctx: context.Background(),
	}, nil
}

// loadCA loads the CA certificate and key from the database.
func (d *PgDepot) loadCA(pass []byte) (*x509.Certificate, *rsa.PrivateKey, error) {
	var pemCert, pemKey []byte
	err := d.db.QueryRowContext(
		d.ctx, `
SELECT
    c.certificate_pem, k.key_pem
FROM
    scep_certificates c
    INNER JOIN scep_ca_keys k ON c.serial = k.serial
WHERE
    c.serial = 1;`,
	).Scan(&pemCert, &pemKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("querying CA: %w", err)
	}

	block, _ := pem.Decode(pemCert)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, nil, errors.New("PEM block not a certificate")
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing certificate: %w", err)
	}

	block, _ = pem.Decode(pemKey)
	if block == nil {
		return nil, nil, errors.New("failed to decode key PEM")
	}

	var keyBytes []byte
	//nolint:staticcheck // x509.IsEncryptedPEMBlock is deprecated but still functional
	if x509.IsEncryptedPEMBlock(block) {
		//nolint:staticcheck // x509.DecryptPEMBlock is deprecated but still functional
		keyBytes, err = x509.DecryptPEMBlock(block, pass)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypting key: %w", err)
		}
	} else {
		keyBytes = block.Bytes
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing private key: %w", err)
	}

	return crt, key, nil
}

// createCA generates a new CA certificate and key and stores it in the database.
func (d *PgDepot) createCA(pass []byte, years int, cn, org, country string) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Insert serial 1 for CA (ignore if exists)
	_, err := d.db.ExecContext(d.ctx, `INSERT INTO scep_serials (serial) VALUES (1) ON CONFLICT DO NOTHING;`)
	if err != nil {
		return nil, nil, fmt.Errorf("inserting CA serial: %w", err)
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generating key: %w", err)
	}

	caCert := depot.NewCACert(
		depot.WithYears(years),
		depot.WithOrganization(org),
		depot.WithCommonName(cn),
		depot.WithCountry(country),
	)
	crtBytes, err := caCert.SelfSign(rand.Reader, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("self-signing CA: %w", err)
	}

	crt, err := x509.ParseCertificate(crtBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	// Store the CA certificate
	if err = d.Put(crt.Subject.CommonName, crt); err != nil {
		return nil, nil, fmt.Errorf("storing CA certificate: %w", err)
	}

	// Encrypt the private key
	//nolint:staticcheck // x509.EncryptPEMBlock is deprecated but still functional
	encPemBlock, err := x509.EncryptPEMBlock(
		rand.Reader,
		"RSA PRIVATE KEY",
		x509.MarshalPKCS1PrivateKey(privKey),
		pass,
		x509.PEMCipher3DES,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypting key: %w", err)
	}

	// Store the encrypted key
	_, err = d.db.ExecContext(
		d.ctx,
		`INSERT INTO scep_ca_keys (serial, key_pem) VALUES ($1, $2);`,
		crt.SerialNumber.Int64(),
		pem.EncodeToMemory(encPemBlock),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("storing CA key: %w", err)
	}

	d.crt = crt
	d.key = privKey
	return d.crt, d.key, nil
}

// CreateOrLoadCA loads an existing CA or creates a new one if none exists.
func (d *PgDepot) CreateOrLoadCA(pass []byte, years int, cn, org, country string) (*x509.Certificate, *rsa.PrivateKey, error) {
	var err error
	d.crt, d.key, err = d.loadCA(pass)
	if err != nil {
		return nil, nil, err
	}
	if d.crt != nil && d.key != nil {
		return d.crt, d.key, nil
	}
	return d.createCA(pass, years, cn, org, country)
}

// CA returns the CA certificate and key.
// Implements depot.Depot interface.
func (d *PgDepot) CA(pass []byte) ([]*x509.Certificate, *rsa.PrivateKey, error) {
	if d.crt == nil || d.key == nil {
		return nil, nil, errors.New("CA certificate or key is empty")
	}
	return []*x509.Certificate{d.crt}, d.key, nil
}

// Put stores a certificate in the database.
// Implements depot.Depot interface.
func (d *PgDepot) Put(name string, crt *x509.Certificate) error {
	// Sanitize name to handle non-UTF8 bytes and null bytes from device CSRs
	name = sanitizeForPg(name)
	if crt.Subject.CommonName == "" {
		// CN was replaced by signature, use hash as name
		name = fmt.Sprintf("%x", sha256.Sum256(crt.Raw))
	}
	if !crt.SerialNumber.IsInt64() {
		return errors.New("cannot represent serial number as int64")
	}
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: crt.Raw,
	}
	_, err := d.db.ExecContext(
		d.ctx, `
INSERT INTO scep_certificates
    (serial, name, not_valid_before, not_valid_after, certificate_pem)
VALUES
    ($1, $2, $3, $4, $5);`,
		crt.SerialNumber.Int64(),
		name,
		crt.NotBefore,
		crt.NotAfter,
		pem.EncodeToMemory(block),
	)
	if err != nil {
		return fmt.Errorf("inserting certificate: %w", err)
	}
	return nil
}

// Serial returns the next available serial number.
// Implements depot.Depot interface.
func (d *PgDepot) Serial() (*big.Int, error) {
	var serial int64
	err := d.db.QueryRowContext(
		d.ctx,
		`INSERT INTO scep_serials DEFAULT VALUES RETURNING serial;`,
	).Scan(&serial)
	if err != nil {
		return nil, fmt.Errorf("generating serial: %w", err)
	}
	return big.NewInt(serial), nil
}

// HasCN checks if a certificate with the given CN exists.
// Implements depot.Depot interface.
func (d *PgDepot) HasCN(cn string, allowTime int, cert *x509.Certificate, revokeOldCertificate bool) (bool, error) {
	// Sanitize CN to handle non-UTF8 bytes and null bytes from device CSRs
	safeCN := sanitizeForPg(cn)
	var ct int
	err := d.db.QueryRowContext(
		d.ctx,
		`SELECT COUNT(*) FROM scep_certificates WHERE name = $1;`,
		safeCN,
	).Scan(&ct)
	if err != nil {
		return false, fmt.Errorf("checking CN: %w", err)
	}
	return ct >= 1, nil
}

// SCEPChallenge generates a new SCEP challenge and stores it in the database.
// Implements challenge.Store interface.
func (d *PgDepot) SCEPChallenge() (string, error) {
	key := make([]byte, 24)
	_, err := rand.Read(key)
	if err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	challenge := base64.StdEncoding.EncodeToString(key)
	_, err = d.db.ExecContext(
		d.ctx,
		`INSERT INTO scep_challenges (challenge) VALUES ($1);`,
		challenge,
	)
	if err != nil {
		return "", fmt.Errorf("storing challenge: %w", err)
	}
	return challenge, nil
}

// HasChallenge verifies and consumes a SCEP challenge.
// Implements challenge.Store interface.
func (d *PgDepot) HasChallenge(pw string) (bool, error) {
	result, err := d.db.ExecContext(
		d.ctx,
		`DELETE FROM scep_challenges WHERE challenge = $1;`,
		pw,
	)
	if err != nil {
		return false, fmt.Errorf("deleting challenge: %w", err)
	}
	rowCt, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected: %w", err)
	}
	if rowCt < 1 {
		return false, errors.New("challenge not found")
	}
	return true, nil
}
