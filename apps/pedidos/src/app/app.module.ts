import { Module } from '@nestjs/common';
import { DatabaseModule } from '../db/database.module';
import { HealthController } from '../health/health.controller';
import { AppController } from './app.controller';
import { AppService } from './app.service';

@Module({
  imports: [DatabaseModule],
  controllers: [AppController, HealthController],
  providers: [AppService],
})
export class AppModule {}
