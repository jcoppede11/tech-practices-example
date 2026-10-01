package idempotency

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func baseRequest() PaymentRequest {
	return PaymentRequest{
		IdempotencyKey: "order-123",
		CustomerID:     "cust-1",
		AmountCents:    4999,
		Currency:       "ARS",
	}
}

// Un reintento con la misma key NO cobra dos veces y devuelve el mismo
// resultado que la operación original.
func TestProcessPayment_RetryIsSafe(t *testing.T) {
	gw := &FakeGateway{}
	svc := NewPaymentService(NewStore(), gw)
	req := baseRequest()

	first, err := svc.ProcessPayment(req)
	if err != nil {
		t.Fatalf("primer intento: error inesperado: %v", err)
	}

	second, err := svc.ProcessPayment(req)
	if err != nil {
		t.Fatalf("reintento: error inesperado: %v", err)
	}

	if got := gw.Charges(); got != 1 {
		t.Fatalf("se esperaba 1 cobro real, hubo %d", got)
	}
	if first.TransactionID != second.TransactionID {
		t.Fatalf("el reintento devolvió otra transacción: %q vs %q",
			first.TransactionID, second.TransactionID)
	}
}

// Bajo concurrencia, muchas peticiones simultáneas con la misma key cobran una sola vez.
func TestProcessPayment_ConcurrentDuplicates(t *testing.T) {
	gw := &FakeGateway{}
	svc := NewPaymentService(NewStore(), gw)
	req := baseRequest()

	const n = 50
	var wg sync.WaitGroup
	var replays atomic.Int64
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := svc.ProcessPayment(req); err == nil {
				replays.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := gw.Charges(); got != 1 {
		t.Fatalf("se esperaba exactamente 1 cobro real, hubo %d", got)
	}
	if replays.Load() < 1 {
		t.Fatalf("al menos una petición debía completarse con éxito")
	}
}

// Reusar la key con otro payload es un conflicto: se rechaza y no genera un
// cobro extra.
func TestProcessPayment_KeyReuseConflict(t *testing.T) {
	gw := &FakeGateway{}
	svc := NewPaymentService(NewStore(), gw)

	if _, err := svc.ProcessPayment(baseRequest()); err != nil {
		t.Fatalf("primer intento: %v", err)
	}

	tampered := baseRequest()
	tampered.AmountCents = 999999

	_, err := svc.ProcessPayment(tampered)
	if !errors.Is(err, ErrKeyReuse) {
		t.Fatalf("se esperaba ErrKeyReuse, se obtuvo %v", err)
	}
	if got := gw.Charges(); got != 1 {
		t.Fatalf("el conflicto no debía generar cobro extra, hubo %d", got)
	}
}

// Sin idempotency key, el servicio rechaza el request antes de cobrar.
func TestProcessPayment_RequiresKey(t *testing.T) {
	gw := &FakeGateway{}
	svc := NewPaymentService(NewStore(), gw)

	req := baseRequest()
	req.IdempotencyKey = ""

	_, err := svc.ProcessPayment(req)
	if !errors.Is(err, ErrIdempotencyKeyRequired) {
		t.Fatalf("se esperaba ErrIdempotencyKeyRequired, se obtuvo %v", err)
	}
	if got := gw.Charges(); got != 0 {
		t.Fatalf("no debían realizarse cobros, se realizaron %d", got)
	}
}
