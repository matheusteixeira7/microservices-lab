import { ServiceUnavailableException } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import { DB } from '../../../shared/infrastructure/database/database.module';
import { HealthController } from './health.controller';

async function createController(execute: () => Promise<unknown>) {
  const module = await Test.createTestingModule({
    controllers: [HealthController],
    providers: [{ provide: DB, useValue: { execute } }],
  }).compile();
  return module.get(HealthController);
}

describe('HealthController', () => {
  it('returns ok when the database answers', async () => {
    const controller = await createController(async () => ({ rows: [] }));
    await expect(controller.check()).resolves.toEqual({ status: 'ok', db: 'up' });
  });

  it('returns 503 when the database is down', async () => {
    const controller = await createController(async () => {
      throw new Error('connect ECONNREFUSED');
    });
    await expect(controller.check()).rejects.toBeInstanceOf(ServiceUnavailableException);
  });
});
