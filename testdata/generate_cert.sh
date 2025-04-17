#!/bin/bash

# Generate a new private key
openssl genrsa -out key.pem 2048

# Generate a certificate signing request with SANs
openssl req -new -key key.pem -out csr.pem -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

# Generate the certificate with SANs
openssl x509 -req -days 365 -in csr.pem -signkey key.pem -out cert.pem -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1")

# Clean up the CSR
rm csr.pem

echo "New certificate and key generated successfully!" 