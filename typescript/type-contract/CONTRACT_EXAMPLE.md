# Contexto

Este ejemplo muestra cómo TypeScript puede actuar como un contrato
entre un productor (backend) y un consumidor (frontend).

Aquí el objetivo no es ejecutar una aplicación, sino demostrar 
qué ocurre cuando el contrato se respeta y qué ocurre cuando se rompe.

## 1. Contrato compartido

📄 Ver: user.dto.ts
Este archivo representa el contrato compartido entre capas.

## 2. Caso válido (el contrato se respeta)
📂 Ver: api/getUser.ts (backend)
📂 Ver: client/renderUser.ts (frontend)

- El contrato se respeta
- El sistema compila
- No hay ambigüedades

## 3. Caso inválido (el contrato se rompe)

// backend
export function getUser(): UserDTO {
  return {
    id: "62bcdc73-46d6-433d-8ebc-d8894e4e3a1d",
    email: "user@email.com",
    status: "active" ❌ 
  }
}

// Error de TypeScript
Type **'"active"'** is not assignable to type 'UserStatus'

## Conclusión

TypeScript ahorrra muchos dolores de cabeza, impide que el sistema
se ejecute cuando el contrato entre productor y consumidor se rompe.

Eso convierte a TypeScript en algo más que tipado:
**lo convierte en un contrato explícito entre sistemas.**
