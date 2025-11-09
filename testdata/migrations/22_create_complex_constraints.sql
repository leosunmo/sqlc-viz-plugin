-- 22_create_complex_constraints.sql
create domain price_range as numeric(10,2) check (
  value > 0 and 
  (value <= 10000 or value in (99999.99, 88888.88))
);

create domain membership_level as text check (
  value in ('bronze', 'silver', 'gold', 'platinum') and
  length(value) >= 4
);

create domain working_hours as integer check (
  value between 0 and 24 and
  not (value in (1, 2, 3, 4, 5)) -- exclude very early morning hours
);

create table subscriptions (
  id serial primary key,
  user_id integer references users(id),
  membership_level membership_level not null,
  monthly_price price_range not null,
  daily_usage_hours working_hours default 8,
  auto_renew boolean default true,
  discount_code slug,
  valid_until timestamptz not null,
  created_at timestamptz default now(),
  constraint subscription_logic check (
    (membership_level = 'bronze' and monthly_price <= 50) or
    (membership_level = 'silver' and monthly_price between 51 and 150) or
    (membership_level = 'gold' and monthly_price between 151 and 500) or
    (membership_level = 'platinum' and monthly_price > 500)
  )
);

---- create above / drop below ----
drop table subscriptions;
drop domain working_hours;
drop domain membership_level;
drop domain price_range;
