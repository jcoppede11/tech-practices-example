/**
 * @fileoverview Función para renderizar información del usuario.
 * Cliente consume el contrato definido por el DTO, en user.dto.ts.
 */

import { UserDTO, UserStatus } from "../user.dto"

export const renderUser = (user: UserDTO): string => {
    if (user.status === UserStatus.ACTIVE) {
        return "Usuario existente"
    }

    return "Usuario inexistente"
}
