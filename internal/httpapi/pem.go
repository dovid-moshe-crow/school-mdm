package httpapi

import (
	"encoding/pem"
	"fmt"
)

func splitCertKeyPEM(in []byte) (certPEM, keyPEM []byte, err error) {
	rest := in
	var certs []byte
	var key []byte
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		encoded := pem.EncodeToMemory(block)
		switch block.Type {
		case "CERTIFICATE":
			certs = append(certs, encoded...)
		case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
			key = append(key, encoded...)
		}
	}
	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no CERTIFICATE PEM found")
	}
	if len(key) == 0 {
		return nil, nil, fmt.Errorf("no PRIVATE KEY PEM found")
	}
	return certs, key, nil
}
