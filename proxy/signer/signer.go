package signer

import (
    "math/big"
    u "net/url"
    "crypto/rand"
    "crypto/rsa"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "time"
    "fmt"
	"log/slog"
    "os"
    "io"
	"github.com/hashicorp/golang-lru/v2"
)

const KeySize int   = 2048
const MaxCacheSize int = 128

type Signer struct {
    Cert   *x509.Certificate
    Pk     *rsa.PrivateKey
    cache  *lru.Cache[string, *tls.Certificate]
}

func (signer *Signer) LoadOrCreate(certPath, keyPath string) error {
    serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
    if err != nil {
        return fmt.Errorf("couldn't generate serial number of a root certificate: %s", err)
    }
    signer.Cert = &x509.Certificate {
        Version: 3,
        SerialNumber: serialNumber,
        Subject: pkix.Name {
            CommonName: "Caching Proxy Root CA",
        },
        NotBefore: time.Now().Add(-24 * time.Hour),
        NotAfter: time.Now().AddDate(10, 0, 0),
        KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
        BasicConstraintsValid: true,
        IsCA: true,
        MaxPathLen: 1,
    }

    if err := signer.LoadPK(keyPath); err == nil {
        return nil
    } else {
        fmt.Println(err)
    }

    if err := signer.GeneratePK(); err != nil {
        return fmt.Errorf("couldn't generate private key: %s", err)
    }

    if err := signer.Save(certPath, keyPath); err != nil {
        return fmt.Errorf("couldn't save private key and certificate: %s", err)
    }

    return nil
}

func (signer *Signer) LoadPK(keyPath string) error {
    pkFile, err := os.Open(keyPath)
    if err != nil {
        return fmt.Errorf("couldn't open private key file: %s", err)
    }

    pkPemBytes, err := io.ReadAll(pkFile)
    if err != nil {
        return fmt.Errorf("couldn't read data from private key file: %s", err)
    }

    pkBlock, _ := pem.Decode(pkPemBytes)
    pkBytes := pkBlock.Bytes

    pk, err := x509.ParsePKCS8PrivateKey(pkBytes)
    if err != nil {
        return fmt.Errorf("couldn't parse private key to PKCS#8: %s", err)
    }
    signer.Pk = pk.(*rsa.PrivateKey)

    return nil
}

func (signer *Signer) Save(certPath, keyPath string) error {
    // Certificate bytes
    certBytes, err := x509.CreateCertificate(
        rand.Reader,
        signer.Cert,
        signer.Cert,
        &signer.Pk.PublicKey,
        signer.Pk,
    )
    if err != nil {
        return fmt.Errorf("couldn't create root certificate: %s", err)
    }

    // Private key bytes
    pkBytes, err := x509.MarshalPKCS8PrivateKey(signer.Pk)
    if err != nil {
        return fmt.Errorf("couldn't marshal PKCS#8 private key: %s", err)
    }

    // Save certificate
    certFile, err := os.Create(certPath)
    if err != nil {
        return fmt.Errorf("couldn't open/create root certificate file: %s", err)
    }
    defer certFile.Close()

    block := pem.Block {
        Type: "CERTIFICATE",
        Bytes: certBytes,
    }

    err = pem.Encode(certFile, &block)
    if err != nil {
        return fmt.Errorf("couldn't write root certificate using PEM encoding to a file: %s", err)
    }

    // Save private key
    keyFile, err := os.Create(keyPath)
    if err != nil {
        return fmt.Errorf("couldn't open/create private key file: %s", err)
    }
    defer keyFile.Close()

    block = pem.Block {
        Type: "RSA PRIVATE KEY",
        Bytes: pkBytes,
    }

    err = pem.Encode(keyFile, &block)
    if err != nil {
        return fmt.Errorf("couldn't write private key using PEM encoding to a file: %s", err)
    }

    return nil
}

func (signer *Signer) GeneratePK() error {
    pk, err := rsa.GenerateKey(rand.Reader, KeySize)
    if err != nil {
        return fmt.Errorf("couldn't generate RSA private key: %s", err)
    }
    signer.Pk = pk
    return nil
}

func (signer *Signer) GenerateCertificate(url u.URL) (*tls.Certificate, error) {
    hostname := url.Hostname()

    if signer.cache == nil {
        cache, err := lru.New[string, *tls.Certificate](MaxCacheSize)
        if err != nil {
            return nil, fmt.Errorf("couldn't initialize LRU cache: %s", err)
        }
        signer.cache = cache
    }

    if signer.cache.Contains(hostname) {
        cert, ok := signer.cache.Get(hostname)
        if ok {
			slog.Debug(fmt.Sprintf("Leaf certificate cache hit for %s", hostname))
            return cert, nil
        }
    }

    pk, err := rsa.GenerateKey(rand.Reader, KeySize)
    if err != nil {
        return nil, fmt.Errorf("couldn't generate private key of a leaf certificate: %s", err)
    }

    serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
    if err != nil {
        return nil, fmt.Errorf("couldn't generate serial number of a leaf certificate: %s", err)
    }
    cert := x509.Certificate {
        Version: 3,
        SerialNumber: serialNumber,
        Subject: pkix.Name {
            CommonName: url.Hostname(),
        },
        NotBefore: time.Now().Add(-time.Hour),
        NotAfter: time.Now().AddDate(1, 0, 0),
        KeyUsage: x509.KeyUsageDigitalSignature,
        ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
        DNSNames: []string{url.Hostname()},
    }

    certBytes, err := x509.CreateCertificate(
        rand.Reader,
        &cert,
        signer.Cert,
        &pk.PublicKey,
        signer.Pk,
    )

    if err != nil {
        return nil, fmt.Errorf("couldn't create leaf certificate: %s", err)
    }

    tlsCert := tls.Certificate {
        Certificate: [][]byte{certBytes},
        PrivateKey: pk,
    }

    signer.cache.Add(hostname, &tlsCert)

    return &tlsCert, nil
}
