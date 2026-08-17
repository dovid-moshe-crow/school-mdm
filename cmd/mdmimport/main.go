// Command mdmimport transforms a nanok/NanoHUB Postgres dump into school-mdm schema mdm.
//
// Expected CSV/JSON is not required: pass a source DATABASE_URL of the old DB and
// the destination school DATABASE_URL. Continuity tables are copied via mdmstore writers.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/mdmstore"
	mdmpg "github.com/dwdmsh/school-mdm/internal/mdmstore/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	srcDSN := flag.String("src", os.Getenv("NANOK_DATABASE_URL"), "source nanok DATABASE_URL")
	dstDSN := flag.String("dst", os.Getenv("DATABASE_URL"), "destination school DATABASE_URL (schema mdm must exist)")
	dryRun := flag.Bool("dry-run", false, "count rows only; do not write")
	flag.Parse()
	if strings.TrimSpace(*srcDSN) == "" || strings.TrimSpace(*dstDSN) == "" {
		log.Fatal("both -src and -dst (or NANOK_DATABASE_URL and DATABASE_URL) are required")
	}

	ctx := context.Background()
	src, err := sql.Open("pgx", *srcDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	if err := src.PingContext(ctx); err != nil {
		log.Fatal("src ping:", err)
	}

	dst, _, err := mdmpg.OpenDSN(ctx, *dstDSN)
	if err != nil {
		log.Fatal("dst:", err)
	}
	defer dst.Close()

	if err := importAll(ctx, src, dst, *dryRun); err != nil {
		log.Fatal(err)
	}
	log.Println("import complete")
}

func importAll(ctx context.Context, src *sql.DB, dst mdmstore.Store, dry bool) error {
	nDev, err := count(ctx, src, `SELECT COUNT(*) FROM devices`)
	if err != nil {
		return fmt.Errorf("count devices (is src a nanok DB?): %w", err)
	}
	nEnr, err := count(ctx, src, `SELECT COUNT(*) FROM enrollments`)
	if err != nil {
		return err
	}
	nAuth, err := count(ctx, src, `SELECT COUNT(*) FROM cert_auth_associations`)
	if err != nil {
		return err
	}
	log.Printf("source: devices=%d enrollments=%d cert_auth=%d dry_run=%v", nDev, nEnr, nAuth, dry)
	if dry {
		return nil
	}

	if err := copyDevices(ctx, src, dst); err != nil {
		return err
	}
	if err := copyEnrollments(ctx, src, dst); err != nil {
		return err
	}
	if err := copyCertAuth(ctx, src, dst); err != nil {
		return err
	}
	if err := copyPushCerts(ctx, src, dst); err != nil {
		return err
	}
	if err := copySCEP(ctx, src, dst); err != nil {
		return err
	}
	return nil
}

func count(ctx context.Context, db *sql.DB, q string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, q).Scan(&n)
	return n, err
}

func copyDevices(ctx context.Context, src *sql.DB, dst mdmstore.Store) error {
	rows, err := src.QueryContext(ctx, `
SELECT id, identity_cert, serial_number, unlock_token, unlock_token_at,
       authenticate, authenticate_at, token_update, token_update_at,
       bootstrap_token_b64, bootstrap_token_at
FROM devices`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d mdmstore.ImportDevice
		var unlockAt, tokenAt, bootAt sql.NullTime
		var identity, serial, tokenUp, boot sql.NullString
		var unlock []byte
		if err := rows.Scan(
			&d.ID, &identity, &serial, &unlock, &unlockAt,
			&d.Authenticate, &d.AuthenticateAt, &tokenUp, &tokenAt,
			&boot, &bootAt,
		); err != nil {
			return err
		}
		if identity.Valid {
			d.IdentityCert = &identity.String
		}
		if serial.Valid {
			d.SerialNumber = &serial.String
		}
		d.UnlockToken = unlock
		if unlockAt.Valid {
			t := unlockAt.Time
			d.UnlockTokenAt = &t
		}
		if tokenUp.Valid {
			d.TokenUpdate = &tokenUp.String
		}
		if tokenAt.Valid {
			t := tokenAt.Time
			d.TokenUpdateAt = &t
		}
		if boot.Valid {
			d.BootstrapTokenB64 = &boot.String
		}
		if bootAt.Valid {
			t := bootAt.Time
			d.BootstrapTokenAt = &t
		}
		if err := dst.ImportDevice(ctx, d); err != nil {
			return fmt.Errorf("device %s: %w", d.ID, err)
		}
	}
	return rows.Err()
}

func copyEnrollments(ctx context.Context, src *sql.DB, dst mdmstore.Store) error {
	rows, err := src.QueryContext(ctx, `
SELECT id, device_id, user_id, type, topic, push_magic, token_hex,
       enabled, token_update_tally, last_seen_at
FROM enrollments`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e mdmstore.ImportEnrollment
		var userID sql.NullString
		if err := rows.Scan(
			&e.ID, &e.DeviceID, &userID, &e.Type, &e.Topic, &e.PushMagic, &e.TokenHex,
			&e.Enabled, &e.TokenUpdateTally, &e.LastSeenAt,
		); err != nil {
			return err
		}
		if userID.Valid {
			e.UserID = &userID.String
		}
		if err := dst.ImportEnrollment(ctx, e); err != nil {
			return fmt.Errorf("enrollment %s: %w", e.ID, err)
		}
	}
	return rows.Err()
}

func copyCertAuth(ctx context.Context, src *sql.DB, dst mdmstore.Store) error {
	rows, err := src.QueryContext(ctx, `SELECT id, sha256 FROM cert_auth_associations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, sha string
		if err := rows.Scan(&id, &sha); err != nil {
			return err
		}
		if err := dst.ImportCertAuth(ctx, id, sha); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyPushCerts(ctx context.Context, src *sql.DB, dst mdmstore.Store) error {
	rows, err := src.QueryContext(ctx, `SELECT topic, cert_pem, key_pem FROM push_certs`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var topic, cert, key string
		if err := rows.Scan(&topic, &cert, &key); err != nil {
			return err
		}
		if err := dst.UpsertPushCert(ctx, topic, []byte(cert), []byte(key)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copySCEP(ctx context.Context, src *sql.DB, dst mdmstore.Store) error {
	// CA (serial 1)
	var certPEM, keyPEM []byte
	err := src.QueryRowContext(ctx, `
SELECT c.certificate_pem, k.key_pem
FROM scep_certificates c
INNER JOIN scep_ca_keys k ON c.serial = k.serial
WHERE c.serial = 1`).Scan(&certPEM, &keyPEM)
	if err == nil {
		if err := dst.ImportSCEPCA(ctx, certPEM, keyPEM); err != nil {
			return fmt.Errorf("scep ca: %w", err)
		}
	} else if err != sql.ErrNoRows {
		return err
	}

	rows, err := src.QueryContext(ctx, `
SELECT serial, name, not_valid_before, not_valid_after, certificate_pem
FROM scep_certificates WHERE serial <> 1`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c mdmstore.ImportSCEPCert
		var before, after time.Time
		if err := rows.Scan(&c.Serial, &c.Name, &before, &after, &c.CertificatePEM); err != nil {
			return err
		}
		c.NotValidBefore = before
		c.NotValidAfter = after
		if err := dst.ImportSCEPCert(ctx, c); err != nil {
			return err
		}
	}
	return rows.Err()
}
