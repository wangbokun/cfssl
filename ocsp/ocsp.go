// Package signer implements certificate signature functionality for CF-SSL.
package ocsp

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"time"

	cferr "github.com/cloudflare/cfssl/errors"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/log"
	"golang.org/x/crypto/ocsp"
)

var statusCode = map[string]int{
	"good":    ocsp.Good,
	"revoked": ocsp.Revoked,
	"unknown": ocsp.Unknown,
}

// SignRequest represents the desired contents of a
// specific OCSP response.
type SignRequest struct {
	Certificate *x509.Certificate
	Status      string
	Reason      int
	RevokedAt   time.Time
}

type Signer interface {
	Sign(req SignRequest) ([]byte, error)
}

// TODO
type StandardSigner struct {
	issuer    *x509.Certificate
	responder *x509.Certificate
	key       crypto.Signer
	interval  time.Duration
}

// NewSignerFromFile reads the issuer cert, the responder cert and the responder key
// from PEM files, and takes an interval in seconds
func NewStandardSignerFromFile(issuerFile, responderFile, keyFile string, interval int) (Signer, error) {
	log.Debug("Loading CA cert: ", issuerFile)
	issuerBytes, err := ioutil.ReadFile(issuerFile)
	if err != nil {
		return nil, err
	}
	log.Debug("Loading responder cert: ", responderFile)
	responderBytes, err := ioutil.ReadFile(issuerFile)
	if err != nil {
		return nil, err
	}
	log.Debug("Loading responder key: ", keyFile)
	keyBytes, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.ReadFailed, err)
	}

	issuerCert, err := helpers.ParseCertificatePEM(issuerBytes)
	if err != nil {
		return nil, err
	}

	responderCert, err := helpers.ParseCertificatePEM(responderBytes)
	if err != nil {
		return nil, err
	}

	key, err := helpers.ParsePrivateKeyPEM(keyBytes)
	if err != nil {
		log.Debug("Malformed private key %v", err)
		return nil, err
	}

	return NewSigner(issuerCert, responderCert, key, interval)
}

// NewSigner simply constructs a new StandardSigner object from the inputs,
// taking the interval in seconds
func NewSigner(issuer, responder *x509.Certificate, key crypto.Signer, interval int) (Signer, error) {
	return &StandardSigner{
		issuer:    issuer,
		responder: responder,
		key:       key,
		interval:  time.Duration(interval) * time.Second,
	}, nil
}

func (s StandardSigner) Sign(req SignRequest) ([]byte, error) {
	if req.Certificate == nil {
		return nil, errors.New("TODO") // XXX
	}

	// Verify that req.Certificate is issued under s.issuer
	if bytes.Compare(req.Certificate.RawIssuer, s.issuer.RawSubject) != 0 {
		return nil, errors.New("TODO") // XXX
	}
	if req.Certificate.CheckSignatureFrom(s.issuer) != nil {
		return nil, errors.New("TODO") // XXX
	}

	// Round thisUpdate times to the nearest hour
	thisUpdate := time.Now().Round(time.Hour)
	nextUpdate := thisUpdate.Add(s.interval)

	status, ok := statusCode[req.Status]
	if !ok {
		return nil, errors.New("TODO") // XXX
	}

	template := ocsp.Response{
		Status:       status,
		SerialNumber: req.Certificate.SerialNumber,
		ThisUpdate:   thisUpdate,
		NextUpdate:   nextUpdate,
		Certificate:  s.responder,
	}

	if status == ocsp.Revoked {
		template.RevokedAt = req.RevokedAt
		template.RevocationReason = req.Reason
	}

	return ocsp.CreateResponse(s.issuer, s.responder, template, s.key)
}
