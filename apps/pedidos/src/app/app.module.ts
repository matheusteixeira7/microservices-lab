import { Module } from '@nestjs/common';
import { HealthModule } from '../modules/health/health.module';
import { DatabaseModule } from '../shared/infrastructure/database/database.module';
import { AppController } from './app.controller';
import { AppService } from './app.service';

@Module({
  imports: [DatabaseModule, HealthModule],
  controllers: [AppController],
  providers: [AppService],
})
export class AppModule {}
