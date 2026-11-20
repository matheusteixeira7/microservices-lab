CREATE TABLE "order_items" (
	"order_id" uuid NOT NULL,
	"position" smallint NOT NULL,
	"sku" text NOT NULL,
	"product_name" text NOT NULL,
	"unit_price_cents" integer NOT NULL,
	"currency" text NOT NULL,
	"quantity" integer NOT NULL,
	CONSTRAINT "order_items_order_id_sku_pk" PRIMARY KEY("order_id","sku"),
	CONSTRAINT "order_items_order_position_unique" UNIQUE("order_id","position"),
	CONSTRAINT "order_items_quantity_check" CHECK ("order_items"."quantity" > 0),
	CONSTRAINT "order_items_unit_price_check" CHECK ("order_items"."unit_price_cents" >= 0)
);
--> statement-breakpoint
CREATE TABLE "orders" (
	"id" uuid PRIMARY KEY NOT NULL,
	"customer_id" text NOT NULL,
	"status" text NOT NULL,
	"cancellation_reason" text,
	"created_at" timestamp with time zone NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "orders_status_check" CHECK ("orders"."status" in ('PENDING', 'STOCK_RESERVED', 'CONFIRMED', 'CANCELLED')),
	CONSTRAINT "orders_cancellation_reason_check" CHECK ("orders"."cancellation_reason" in ('OUT_OF_STOCK', 'PAYMENT_DECLINED', 'CUSTOMER_REQUESTED')),
	CONSTRAINT "orders_cancellation_reason_matches_status" CHECK (("orders"."status" = 'CANCELLED') = ("orders"."cancellation_reason" is not null))
);
--> statement-breakpoint
CREATE TABLE "outbox_events" (
	"sequence" bigint GENERATED ALWAYS AS IDENTITY (sequence name "outbox_events_sequence_seq" INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 START WITH 1 CACHE 1),
	"id" uuid PRIMARY KEY NOT NULL,
	"aggregate_type" text NOT NULL,
	"aggregate_id" text NOT NULL,
	"event_type" text NOT NULL,
	"payload" jsonb NOT NULL,
	"occurred_at" timestamp with time zone NOT NULL,
	"published_at" timestamp with time zone
);
--> statement-breakpoint
ALTER TABLE "order_items" ADD CONSTRAINT "order_items_order_id_orders_id_fk" FOREIGN KEY ("order_id") REFERENCES "public"."orders"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "outbox_events_unpublished_idx" ON "outbox_events" USING btree ("sequence") WHERE "outbox_events"."published_at" is null;