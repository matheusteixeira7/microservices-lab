import { sql } from 'drizzle-orm';
import { bigint, index, jsonb, pgTable, text, timestamp, uuid } from 'drizzle-orm/pg-core';

/**
 * Transactional outbox: domain events are written here in the SAME transaction
 * as the aggregate, so a saved change never loses its events.
 *
 * Phase 2 adds the relay that reads unpublished rows (in `sequence` order),
 * sends them to Kafka and fills `published_at`.
 */
export const outboxEvents = pgTable(
  'outbox_events',
  {
    /** Stable publishing order for the relay. */
    sequence: bigint('sequence', { mode: 'number' }).generatedAlwaysAsIdentity(),
    /** Unique event id: consumers use it to ignore duplicates. */
    id: uuid('id').primaryKey(),
    aggregateType: text('aggregate_type').notNull(),
    aggregateId: text('aggregate_id').notNull(),
    eventType: text('event_type').notNull(),
    payload: jsonb('payload').notNull(),
    occurredAt: timestamp('occurred_at', { withTimezone: true }).notNull(),
    publishedAt: timestamp('published_at', { withTimezone: true }),
  },
  (t) => [
    // The relay only looks at unpublished events.
    index('outbox_events_unpublished_idx').on(t.sequence).where(sql`${t.publishedAt} is null`),
  ],
);
