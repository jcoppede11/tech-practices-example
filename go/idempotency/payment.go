// Package idempotency demuestra cómo garantizar que una operación de pago
// se ejecute UNA sola vez, aun cuando el cliente la reintente varias veces
// (timeouts de red, doble clic, reintentos automáticos de un SDK, etc.).
package idempotency

import (
	"errors"
	"fmt"
)

// Errores del contrato de idempotencia.
var (
	// ErrIdempotencyKeyRequired: el request no trae idempotency key.
	ErrIdempotencyKeyRequired = errors.New("pago: idempotency key requerida")
	// ErrRequestInProgress: llegó un duplicado mientras el original aún se procesa.
	ErrRequestInProgress = errors.New("pago: operación en curso para esta idempotency key")
	// ErrKeyReuse: se reutilizó una idempotency key con un payload distinto.
	ErrKeyReuse = errors.New("pago: idempotency key reutilizada con otro payload")
)

// PaymentRequest es el cobro que envía el cliente.
//
//   - IdempotencyKey identifica la operación lógica. La genera el cliente
//     (p. ej. un UUID v4 por intento de compra) y la repite idéntica en cada
//     reintento.
//   - AmountCents expresa el monto en centavos (entero) para evitar los errores
//     de redondeo del punto flotante en dinero.
type PaymentRequest struct {
	IdempotencyKey string
	CustomerID     string
	AmountCents    int64
	Currency       string
}

// PaymentResult es el resultado del cobro.
type PaymentResult struct {
	TransactionID string
	Status        string
	AmountCents   int64
	Currency      string
}

// PaymentService orquesta el cobro. Depende de una Store (dónde se recuerda
// cada key) y de un PaymentGateway (el efecto colateral que no queremos
// ejecutar dos veces).
type PaymentService struct {
	store   *Store
	gateway PaymentGateway
}

// NewPaymentService construye el servicio con sus dependencias inyectadas.
func NewPaymentService(store *Store, gateway PaymentGateway) *PaymentService {
	return &PaymentService{store: store, gateway: gateway}
}

// ProcessPayment ejecuta el cobro exactamente UNA vez por idempotency key,
// siguiendo el mismo patrón que en producción:
//
//  1. Reservar la key (detecta reintento o conflicto).
//  2. Ejecutar el cobro real; si falla, liberar la key para reintentar limpio.
//  3. Persistir el resultado para los futuros reintentos.
func (s *PaymentService) ProcessPayment(req PaymentRequest) (PaymentResult, error) {
	if req.IdempotencyKey == "" {
		return PaymentResult{}, ErrIdempotencyKeyRequired
	}

	prev, replay, err := s.store.begin(req.IdempotencyKey, fingerprint(req))
	if err != nil {
		return PaymentResult{}, err
	}
	if replay {
		return *prev, nil
	}

	result, err := s.gateway.Charge(req)
	if err != nil {
		s.store.rollback(req.IdempotencyKey)
		return PaymentResult{}, fmt.Errorf("pago: falló el cobro: %w", err)
	}

	s.store.complete(req.IdempotencyKey, result)
	return result, nil
}

// fingerprint genera una huella estable del contenido del request, para
// detectar que una misma key se reuse con parámetros distintos.
func fingerprint(req PaymentRequest) string {
	return fmt.Sprintf("%s|%d|%s", req.CustomerID, req.AmountCents, req.Currency)
}
