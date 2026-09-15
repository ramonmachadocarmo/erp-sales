INSERT INTO sales_outbox_events (aggregate_type, aggregate_id, event_type, payload, status)
SELECT 'SALES_ORDER', o.id::text, 'sales.order.updated',
  jsonb_build_object(
    'order_id', o.id,
    'warehouse_id', COALESCE(pk.warehouse_id::text, o.warehouse_id::text, ''),
    'customer_id', o.customer_id,
    'items', COALESCE((
      SELECT jsonb_agg(jsonb_build_object(
        'product_id', i.product_id,
        'quantity', i.quantity,
        'unit_price', i.unit_price,
        'subtotal', i.subtotal
      ))
      FROM sales_order_items i
      WHERE i.sales_order_id = o.id
    ), '[]'::jsonb),
    'total_amount', o.total_amount
  ),
  'PENDING'
FROM sales_orders o
LEFT JOIN sales_pickings pk ON pk.sales_order_id = o.id
WHERE o.status IN ('APPROVED', 'PENDING_RESERVATION', 'PICKING', 'PICKED', 'DELIVERED');
