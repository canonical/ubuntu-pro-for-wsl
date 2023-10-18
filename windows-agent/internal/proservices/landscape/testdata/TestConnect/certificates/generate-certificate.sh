#!/bin/bash

openssl req                                                 \
    -x509                                                   \
    -newkey rsa:4096                                        \
    -keyout key.pem                                         \
    -out cert.pem                                           \
    -sha256                                                 \
    -nodes                                                  \
    -addext 'subjectAltName = IP:127.0.0.1'                 \
    -subj "/C=US/O=Canonical/CN=CanonicalGroupLimited"

echo This is not a valid certificate > bad-certificate.pem