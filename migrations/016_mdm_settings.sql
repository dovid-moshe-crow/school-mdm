-- Singleton MDM/ABM settings (DEP slot name lives here, not in env).

CREATE TABLE IF NOT EXISTS mdm_settings (
    id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    dep_name   TEXT NOT NULL DEFAULT 'nanok',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT mdm_settings_dep_name_len CHECK (char_length(dep_name) BETWEEN 1 AND 64)
);

INSERT INTO mdm_settings (id, dep_name)
VALUES (1, 'nanok')
ON CONFLICT (id) DO NOTHING;
