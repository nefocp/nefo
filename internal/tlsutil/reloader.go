package tlsutil

import (
	"crypto/tls"
	"os"
	"sync"
	"time"
)

// Pair is a certificate and its private key on disk.
type Pair struct{ Cert, Key string }

// Reloader serves the first certificate pair that exists and loads, and
// notices new or renewed files within a minute, without a restart.
type Reloader struct {
	pairs   []Pair
	mu      sync.Mutex
	current *tls.Certificate
	from    Pair
	mod     time.Time
	checked time.Time
}

// NewReloader returns a Reloader trying pairs in order.
func NewReloader(pairs ...Pair) (*Reloader, error) {
	r := &Reloader{pairs: pairs}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Reloader) reload() error {
	var firstErr error
	for _, p := range r.pairs {
		fi, err := os.Stat(p.Cert)
		if err != nil {
			continue
		}
		if r.current != nil && p == r.from && fi.ModTime().Equal(r.mod) {
			return nil // unchanged
		}
		c, err := tls.LoadX509KeyPair(p.Cert, p.Key)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue // a half-written renewal: keep looking
		}
		r.current, r.from, r.mod = &c, p, fi.ModTime()
		return nil
	}
	if r.current != nil {
		return nil
	}
	if firstErr == nil {
		firstErr = os.ErrNotExist
	}
	return firstErr
}

// GetCertificate is used as tls.Config.GetCertificate.
func (r *Reloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.checked) > time.Minute {
		r.checked = time.Now()
		r.reload()
	}
	return r.current, nil
}

// Source returns the certificate file in use.
func (r *Reloader) Source() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.from.Cert
}
