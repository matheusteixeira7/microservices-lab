import { ArgumentsHost, Catch, ExceptionFilter, HttpStatus } from '@nestjs/common';
import { DomainError } from '../domain/domain-error';

/** The tiny part of the Express response we use (avoids depending on @types/express). */
interface JsonResponse {
  status(code: number): { json(body: unknown): void };
}

/**
 * The HTTP status of each domain error lives HERE, not in the domain
 * (the domain does not know HTTP exists).
 */
const STATUS_BY_CODE: Record<string, HttpStatus> = {
  ORDER_NOT_FOUND: HttpStatus.NOT_FOUND,
  INVALID_ORDER_TRANSITION: HttpStatus.CONFLICT,
};

/**
 * Turns DomainErrors into 4xx responses with `{ code, message }`.
 * Only DomainError is caught: Nest's own HttpExceptions (400, 503...) and
 * unexpected errors (500, no details leaked) keep their default handling.
 */
@Catch(DomainError)
export class DomainErrorFilter implements ExceptionFilter {
  catch(error: DomainError, host: ArgumentsHost): void {
    const status = STATUS_BY_CODE[error.code] ?? HttpStatus.UNPROCESSABLE_ENTITY;
    host
      .switchToHttp()
      .getResponse<JsonResponse>()
      .status(status)
      .json({ code: error.code, message: error.message });
  }
}
