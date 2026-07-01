// Package catalog is the merchant product catalog: barcodes → price, managed by the
// store owner in the dashboard. The customer's scan-and-go reads it by barcode
// (ADR-0007) to build a cart; charges/payment are settled elsewhere. In-memory for the
// demo. Money is integer minor units (kuruş, ADR-0003).
package catalog

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
)

// Product is one catalog item.
type Product struct {
	ID         string `json:"id"`
	MerchantID string `json:"merchant_id"`
	Barcode    string `json:"barcode"`
	Name       string `json:"name"`
	PriceMinor int64  `json:"price_minor"`
}

var (
	ErrNotFound     = errors.New("catalog: product not found")
	ErrBadProduct   = errors.New("catalog: name, barcode and positive price required")
	ErrBarcodeTaken = errors.New("catalog: barcode already exists for this merchant")
)

// Store holds products in memory, keyed by id, with a merchant+barcode index for scans.
type Store struct {
	mu       sync.RWMutex
	products map[string]*Product
}

func NewStore(seed ...Product) *Store {
	s := &Store{products: make(map[string]*Product)}
	for _, p := range seed {
		cp := p
		if cp.ID == "" {
			cp.ID = newID()
		}
		s.products[cp.ID] = &cp
	}
	return s
}

func (s *Store) ForMerchant(merchantID string) []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Product
	for _, p := range s.products {
		if p.MerchantID == merchantID {
			out = append(out, *p)
		}
	}
	return out
}

func (s *Store) Create(merchantID, barcode, name string, priceMinor int64) (Product, error) {
	if name == "" || barcode == "" || priceMinor <= 0 {
		return Product{}, ErrBadProduct
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.products {
		if p.MerchantID == merchantID && p.Barcode == barcode {
			return Product{}, ErrBarcodeTaken
		}
	}
	p := &Product{ID: newID(), MerchantID: merchantID, Barcode: barcode, Name: name, PriceMinor: priceMinor}
	s.products[p.ID] = p
	return *p, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[id]; !ok {
		return ErrNotFound
	}
	delete(s.products, id)
	return nil
}

// ByBarcode resolves a scanned barcode to a product (scan-and-go price lookup).
func (s *Store) ByBarcode(barcode string) (Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.products {
		if p.Barcode == barcode {
			return *p, nil
		}
	}
	return Product{}, ErrNotFound
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "prod_" + hex.EncodeToString(b[:])
}
