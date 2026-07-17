// Точка входа в бэкенд Manibandha. Токен получаем после входа (SMS-логин как в вебе).
export const API_BASE = 'https://manibandha.prema.su';

let token: string | null = null;
export function setToken(t: string | null): void { token = t; }
export function getToken(): string | null { return token; }
