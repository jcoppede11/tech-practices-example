/**
 * @fileoverview Función para obtener información del usuario.
 * Backend funciona como proveedor de 
 * datos, según el contrato definido por el DTO, en user.dto.ts.
 */

import { UserDTO, UserStatus } from "../user.dto"

export const getUser = (): UserDTO => {
    return {
        id: "62bcdc73-46d6-433d-8ebc-d8894e4e3a1d",
        email: "user@email.com",
        status: UserStatus.ACTIVE,
    }
}
