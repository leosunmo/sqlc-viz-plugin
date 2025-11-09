-- 25_create_financial_records.sql
create domain currency_code as text check (value ~ '^[A-Z]{3}$');
create domain money_amount as numeric(15,2) check (value >= -999999999999.99 and value <= 999999999999.99);

create table financial_transactions (
  id serial primary key,
  user_id integer references users(id),
  amount money_amount not null,
  currency currency_code default 'USD',
  transaction_type text not null,
  description short_text,
  processed_at timestamptz,
  created_at timestamptz default now(),
  constraint valid_transaction_type check (
    transaction_type in ('credit', 'debit', 'transfer', 'refund')
  ),
  constraint amount_sign_check check (
    (transaction_type in ('credit', 'refund') and amount > 0) or
    (transaction_type in ('debit', 'transfer') and amount < 0)
  ),
  constraint processing_time_check check (
    processed_at is null or processed_at >= created_at
  )
);

create table account_balances (
  id serial primary key,
  user_id integer references users(id) unique,
  current_balance money_amount default 0.00,
  currency currency_code default 'USD',
  credit_limit money_amount check (credit_limit >= 0),
  last_updated timestamptz default now(),
  constraint balance_logic check (
    (current_balance >= 0 and credit_limit is null) or
    (current_balance >= -credit_limit and credit_limit is not null)
  )
);

create view user_financial_summary as
select 
  u.id,
  u.username,
  ab.current_balance,
  ab.currency,
  ab.credit_limit,
  count(ft.id) as transaction_count,
  max(ft.created_at) as last_transaction_date
from users u
left join account_balances ab on u.id = ab.user_id
left join financial_transactions ft on u.id = ft.user_id
group by u.id, u.username, ab.current_balance, ab.currency, ab.credit_limit;

---- create above / drop below ----
drop view user_financial_summary;
drop table account_balances;
drop table financial_transactions;
drop domain money_amount;
drop domain currency_code;
