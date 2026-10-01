# Contexto

Este ejemplo muestra cómo garantizar **idempotencia** en un sistema de
procesamiento de pagos escrito en Go.

Una operación es **idempotente** cuando ejecutarla una vez o muchas veces
produce el mismo efecto. En pagos esto es crítico: la red falla, el usuario
hace doble clic y los SDKs reintentan automáticamente. Sin idempotencia, un
mismo cobro podría ejecutarse dos veces.

El objetivo no es correr una aplicación completa, sino demostrar el patrón
y las decisiones de diseño detrás de él.

## El problema

```
Cliente  ──POST /pay──▶  Servidor  ──cobra──▶  Pasarela ✅
Cliente  ◀─ ⚡timeout ──  (la respuesta se pierde en la red)
Cliente  ──POST /pay──▶  Servidor  ──cobra──▶  Pasarela ❌ (doble cobro)
```

El cliente nunca supo que el primer cobro funcionó, así que reintenta. Sin
protección, el cliente termina pagando dos veces.

## La solución: idempotency key

El cliente genera un identificador único por operación lógica (p. ej. un UUID
por intento de compra) y lo **repite idéntico en cada reintento**. El servidor
lo usa para distinguir "operación nueva" de "reintento".

## 1. El contrato

📄 Ver: `payment.go`

- `PaymentRequest` lleva la `IdempotencyKey` (la genera el cliente).
- `PaymentResult` es el resultado que se guarda y se reutiliza en los reintentos.
- Los montos van en **centavos** (`int64`), nunca en `float`, para evitar
  errores de redondeo en dinero.

## 2. El flujo idempotente

📂 Ver: `payment.go` → `ProcessPayment`
📂 Ver: `store.go` → `begin` / `complete` / `rollback`

```
ProcessPayment(req)
   │
   ├─ 1. store.begin(key, fingerprint)   ← reserva atómica
   │        ├─ primera vez     → sigue al cobro
   │        ├─ ya completada   → devuelve el resultado guardado (sin cobrar)
   │        ├─ en curso        → ErrRequestInProgress
   │        └─ otro payload    → ErrKeyReuse
   │
   ├─ 2. gateway.Charge(req)              ← efecto colateral, UNA sola vez
   │        └─ si falla → store.rollback(key) (permite reintentar)
   │
   └─ 3. store.complete(key, result)      ← guarda el resultado
```

## 3. Casos demostrados (en los tests)

📂 Ver: `payment_test.go`

| Caso | Qué se verifica |
|------|-----------------|
| **Reintento idéntico** | El 2º llamado NO cobra de nuevo; devuelve el mismo `TransactionID`. |
| **Duplicados concurrentes** | 50 peticiones simultáneas con la misma key → **un solo** cobro real. |
| **Key reutilizada con otro payload** | Se rechaza con `ErrKeyReuse`, sin cobro extra. |
| **Sin key** | Se rechaza con `ErrIdempotencyKeyRequired` antes de cobrar. |

```bash
cd go/idempotency
go test -race -v ./...
```

> El flag `-race` es clave acá: detecta condiciones de carrera en el acceso
> concurrente a la Store.

## Best practices aplicadas

- **Idempotency key provista por el cliente**: identifica la operación lógica,
  no el request HTTP.
- **Reserva atómica** (`begin`): en memoria es un mutex; en producción sería la
  `PRIMARY KEY` de una tabla, que el motor garantiza bajo concurrencia.
- **Estado explícito** (`en curso` / `completada`): distingue un reintento
  seguro de un duplicado que todavía está corriendo.
- **Fingerprint del request**: detecta el reuso de una key con datos distintos.
- **Rollback ante fallo**: un cobro fallido libera la key y permite reintentar,
  en vez de dejarla bloqueada para siempre.
- **Dinero en enteros** (centavos), nunca `float`.
- **Inversión de dependencias**: `PaymentGateway` es una interfaz, lo que
  permite testear con un `FakeGateway` que cuenta los cobros.

## Trade-offs y límites de este ejemplo

El patrón es correcto, pero la `Store` en memoria es deliberadamente simple
para mantener el ejemplo autocontenido. Estos son los límites reales y cómo se
resuelven en producción:

| Límite de esta implementación | Consecuencia | Cómo se resuelve en producción |
|-------------------------------|--------------|--------------------------------|
| **No es durable** (mapa en RAM) | Si el proceso se reinicia, se pierden todas las keys. | Persistir en una tabla con la idempotency key como `PRIMARY KEY`; el motor garantiza unicidad y atomicidad. |
| **El estado `en curso` no expira** | La key queda bloqueada para siempre con `ErrRequestInProgress`. | Lease con TTL: la reserva vence tras N segundos y un reintento puede retomarla. |
| **Las keys nunca se borran** | El mapa crece sin límite (fuga de memoria). | Expiración (TTL de 24–72 h) sobre la fila de idempotencia. |
| **Un solo proceso** | El mutex sólo coordina goroutines dentro de esta instancia. | Con varias instancias, la unicidad la da la base de datos (la `PRIMARY KEY`), no el mutex. |
| **`rollback` ante fallo del gateway** | Asume que el cobro no ocurrió. Si el gateway cobró pero falló la respuesta, liberar la key podría recobrar. | Guardar el intento como `fallido` y reconciliar contra el estado real del gateway antes de reintentar. |

> La regla de fondo: **la garantía de unicidad debe vivir en el almacenamiento,
> no en el proceso.** Acá el mutex cumple ese rol sólo porque hay un único
> proceso y una única instancia.

## Conclusión

La idempotencia no se logra con un solo `if`: es un **contrato de ejecución**.
La idempotency key, el estado explícito y la reserva atómica convierten un
"reintentá y rezá" en una garantía:

**el dinero se cobra exactamente una vez, pase lo que pase con la red.**
