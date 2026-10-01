package idempotency

import (
	"fmt"
	"sync/atomic"
)

// PaymentGateway abstrae al proveedor externo que realiza el cobro real
// (Stripe, Mercado Pago, una pasarela bancaria, etc.).
//
// Definir una interfaz permite inyectar una implementación de prueba en los
// tests (inversión de dependencias).
type PaymentGateway interface {
	Charge(req PaymentRequest) (PaymentResult, error)
}

// FakeGateway es una implementación de prueba que cuenta cuántos cobros reales
// ejecutó. Gracias a ese contador podemos demostrar que la idempotencia evita
// el doble cobro. Debe usarse siempre por puntero.
type FakeGateway struct {
	charges atomic.Int64
}

// Charge simula el cobro: incrementa el contador y devuelve una transacción.
func (g *FakeGateway) Charge(req PaymentRequest) (PaymentResult, error) {
	n := g.charges.Add(1)
	return PaymentResult{
		TransactionID: fmt.Sprintf("txn_%d", n),
		Status:        "SUCCEEDED",
		AmountCents:   req.AmountCents,
		Currency:      req.Currency,
	}, nil
}

// Charges devuelve cuántos cobros reales se ejecutaron.
func (g *FakeGateway) Charges() int64 {
	return g.charges.Load()
}
