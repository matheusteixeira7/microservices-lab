import { Module } from '@nestjs/common';
import { HealthModule } from '../modules/health/health.module';
import { OrdersModule } from '../modules/orders/orders.module';
import { DatabaseModule } from '../shared/infrastructure/database/database.module';
import { ErrorHandlingModule } from '../shared/http/error-handling.module';
import { AppController } from './app.controller';
import { AppService } from './app.service';

@Module({
  imports: [DatabaseModule, ErrorHandlingModule, HealthModule, OrdersModule],
  controllers: [AppController],
  providers: [AppService],
})
export class AppModule {}
