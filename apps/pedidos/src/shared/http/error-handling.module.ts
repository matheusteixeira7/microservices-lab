import { Module } from '@nestjs/common';
import { APP_FILTER } from '@nestjs/core';
import { DomainErrorFilter } from './domain-error.filter';

/**
 * Registers the global error filter. Imported by AppModule and by the HTTP
 * tests, so tests see exactly the same error responses as production.
 */
@Module({
  providers: [{ provide: APP_FILTER, useClass: DomainErrorFilter }],
})
export class ErrorHandlingModule {}
