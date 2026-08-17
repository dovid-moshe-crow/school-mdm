-- MDM protocol storage in isolated schema `mdm` (avoids collision with school public.devices).
-- Table shapes match nanomdm/nanok for transform import of continuity data.

CREATE SCHEMA IF NOT EXISTS mdm;

CREATE TABLE IF NOT EXISTS mdm.devices (
    id                  VARCHAR(255) NOT NULL PRIMARY KEY,
    identity_cert       TEXT         NULL,
    serial_number       VARCHAR(127) NULL,
    unlock_token        BYTEA        NULL,
    unlock_token_at     TIMESTAMPTZ  NULL,
    authenticate        TEXT         NOT NULL,
    authenticate_at     TIMESTAMPTZ  NOT NULL,
    token_update        TEXT         NULL,
    token_update_at     TIMESTAMPTZ  NULL,
    bootstrap_token_b64 TEXT         NULL,
    bootstrap_token_at  TIMESTAMPTZ  NULL,
    created_at          TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS mdm_devices_serial_number ON mdm.devices (serial_number);

CREATE TABLE IF NOT EXISTS mdm.users (
    id                          VARCHAR(255) NOT NULL,
    device_id                   VARCHAR(255) NOT NULL REFERENCES mdm.devices (id) ON DELETE CASCADE ON UPDATE CASCADE,
    user_short_name             VARCHAR(255) NULL,
    user_long_name              VARCHAR(255) NULL,
    token_update                TEXT         NULL,
    token_update_at             TIMESTAMPTZ  NULL,
    user_authenticate           TEXT         NULL,
    user_authenticate_at        TIMESTAMPTZ  NULL,
    user_authenticate_digest    TEXT         NULL,
    user_authenticate_digest_at TIMESTAMPTZ  NULL,
    created_at                  TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    updated_at                  TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, device_id),
    UNIQUE (id)
);

CREATE TABLE IF NOT EXISTS mdm.enrollments (
    id                 VARCHAR(255) NOT NULL PRIMARY KEY,
    device_id          VARCHAR(255) NOT NULL REFERENCES mdm.devices (id) ON DELETE CASCADE ON UPDATE CASCADE,
    user_id            VARCHAR(255) NULL UNIQUE,
    type               VARCHAR(31)  NOT NULL,
    topic              VARCHAR(255) NOT NULL,
    push_magic         VARCHAR(127) NOT NULL,
    token_hex          VARCHAR(255) NOT NULL,
    enabled            BOOLEAN      NOT NULL DEFAULT TRUE,
    token_update_tally INTEGER      NOT NULL DEFAULT 1,
    last_seen_at       TIMESTAMPTZ  NOT NULL,
    created_at         TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS mdm_enrollments_type ON mdm.enrollments (type);
CREATE INDEX IF NOT EXISTS mdm_enrollments_enabled ON mdm.enrollments (enabled);

CREATE TABLE IF NOT EXISTS mdm.commands (
    command_uuid VARCHAR(127) NOT NULL PRIMARY KEY,
    request_type VARCHAR(63)  NOT NULL,
    command      TEXT         NOT NULL,
    created_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mdm.command_results (
    id            VARCHAR(255) NOT NULL REFERENCES mdm.enrollments (id) ON DELETE CASCADE ON UPDATE CASCADE,
    command_uuid  VARCHAR(127) NOT NULL REFERENCES mdm.commands (command_uuid) ON DELETE CASCADE ON UPDATE CASCADE,
    status        VARCHAR(31)  NOT NULL,
    result        TEXT         NOT NULL,
    not_now_at    TIMESTAMPTZ  NULL,
    not_now_tally INTEGER      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, command_uuid)
);

CREATE INDEX IF NOT EXISTS mdm_command_results_status ON mdm.command_results (status);

CREATE TABLE IF NOT EXISTS mdm.enrollment_queue (
    id           VARCHAR(255) NOT NULL REFERENCES mdm.enrollments (id) ON DELETE CASCADE ON UPDATE CASCADE,
    command_uuid VARCHAR(127) NOT NULL REFERENCES mdm.commands (command_uuid) ON DELETE CASCADE ON UPDATE CASCADE,
    active       BOOLEAN      NOT NULL DEFAULT TRUE,
    priority     SMALLINT     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, command_uuid)
);

CREATE INDEX IF NOT EXISTS mdm_enrollment_queue_priority ON mdm.enrollment_queue (priority DESC, created_at);

CREATE OR REPLACE VIEW mdm.view_queue AS
SELECT q.id,
       q.created_at,
       q.active,
       q.priority,
       c.command_uuid,
       c.request_type,
       c.command,
       r.updated_at AS result_updated_at,
       r.status,
       r.result
FROM mdm.enrollment_queue AS q
INNER JOIN mdm.commands AS c ON q.command_uuid = c.command_uuid
LEFT JOIN mdm.command_results r ON r.command_uuid = q.command_uuid AND r.id = q.id
ORDER BY q.priority DESC, q.created_at;

CREATE TABLE IF NOT EXISTS mdm.push_certs (
    topic       VARCHAR(255) NOT NULL PRIMARY KEY,
    cert_pem    TEXT         NOT NULL,
    key_pem     TEXT         NOT NULL,
    stale_token INTEGER      NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mdm.cert_auth_associations (
    id         VARCHAR(255) NOT NULL,
    sha256     CHAR(64)     NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, sha256)
);

CREATE TABLE IF NOT EXISTS mdm.scep_serials (
    serial BIGSERIAL PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS mdm.scep_certificates (
    serial BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    not_valid_before TIMESTAMPTZ NOT NULL,
    not_valid_after TIMESTAMPTZ NOT NULL,
    certificate_pem TEXT NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS mdm_scep_certificates_name ON mdm.scep_certificates (name);

CREATE TABLE IF NOT EXISTS mdm.scep_ca_keys (
    serial BIGINT PRIMARY KEY REFERENCES mdm.scep_certificates (serial),
    key_pem TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS mdm.scep_challenges (
    id BIGSERIAL PRIMARY KEY,
    challenge TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION mdm.update_current_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.devices;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.devices
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.users;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.users
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.enrollments;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.enrollments
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.commands;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.commands
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.command_results;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.command_results
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.enrollment_queue;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.enrollment_queue
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.push_certs;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.push_certs
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();

DROP TRIGGER IF EXISTS update_at_to_current_timestamp ON mdm.cert_auth_associations;
CREATE TRIGGER update_at_to_current_timestamp BEFORE UPDATE ON mdm.cert_auth_associations
    FOR EACH ROW EXECUTE PROCEDURE mdm.update_current_timestamp();
