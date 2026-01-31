/**
 * DTO: Data Transfer Object
 * Contrato entre capas para la transferencia de datos.
 */

export enum UserStatus {
    ACTIVE = "ACTIVE",
    SUSPENDED = "SUSPENDED",
    PENDING = "PENDING",
}

export interface UserDTO {
    id: string
    email: string
    status: UserStatus
}
