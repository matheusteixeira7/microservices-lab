import * as path from 'path';
import type { FastifyInstance } from 'fastify';
import AutoLoad from '@fastify/autoload';

/* eslint-disable-next-line */
export interface AppOptions { }

// Autoload carrega todo arquivo da pasta. Sem este filtro, ele carregaria os
// testes (*.spec.ts) como se fossem plugins e rotas.
const ignoreTests = /\.(spec|test)\.[cm]?[jt]s$/;

export async function app(fastify: FastifyInstance, opts: AppOptions) {
  // This loads all plugins defined in plugins
  // those should be support plugins that are reused
  // through your application
  fastify.register(AutoLoad, {
    dir: path.join(import.meta.dirname, 'plugins'),
    ignoreFilter: ignoreTests,
    options: { ...opts },
  });

  // This loads all plugins defined in routes
  // define your routes in one of these
  fastify.register(AutoLoad, {
    dir: path.join(import.meta.dirname, 'routes'),
    ignoreFilter: ignoreTests,
    options: { ...opts },
  });
}
