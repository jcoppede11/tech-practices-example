package idempotency

import "sync"

// recordState modela el ciclo de vida de una operación idempotente.
type recordState int

const (
	stateInProgress recordState = iota // reservada, el cobro aún corre
	stateCompleted                     // terminada, el resultado está guardado
)

type record struct {
	state       recordState
	fingerprint string
	result      *PaymentResult
}

// Store recuerda el estado de cada operación idempotente.
//
// En producción esto sería una tabla con la idempotency key como PRIMARY KEY.
// Aquí lo modelamos con un mapa protegido por un mutex para mantener el ejemplo
// autocontenido.
type Store struct {
	mu      sync.Mutex
	records map[string]*record
}

// NewStore crea una Store vacía.
func NewStore() *Store {
	return &Store{records: make(map[string]*record)}
}

// begin reserva la key para una operación nueva de forma atómica y, según el
// estado previo, devuelve una de tres situaciones:
//
//   - Primera vez: la key queda reservada; (nil, false, nil). El llamador DEBE
//     procesar el cobro y luego invocar complete.
//   - Reintento ya completado con el mismo payload: (result, true, nil). El
//     llamador devuelve ese resultado sin cobrar de nuevo.
//   - Conflicto (ErrKeyReuse) o duplicado concurrente (ErrRequestInProgress):
//     (nil, false, err).
func (s *Store) begin(key, fingerprint string) (*PaymentResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, exists := s.records[key]
	if !exists {
		s.records[key] = &record{state: stateInProgress, fingerprint: fingerprint}
		return nil, false, nil
	}

	if rec.fingerprint != fingerprint {
		return nil, false, ErrKeyReuse
	}

	switch rec.state {
	case stateCompleted:
		return rec.result, true, nil
	default:
		return nil, false, ErrRequestInProgress
	}
}

// complete marca la operación como terminada y guarda su resultado para que
// los reintentos posteriores lo reutilicen.
func (s *Store) complete(key string, result PaymentResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.records[key]; ok {
		rec.state = stateCompleted
		rec.result = &result
	}
}

// rollback libera una key reservada cuyo cobro falló, para permitir un
// reintento limpio. Sin esto, un cobro fallido dejaría la key en curso para
// siempre y bloquearía reintentos legítimos.
func (s *Store) rollback(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, key)
}
