-- 19_create_product_catalog.sql
create type product_category as enum ('electronics', 'clothing', 'books', 'home', 'toys', 'sports');
create type size_category as enum ('xs', 's', 'm', 'l', 'xl', 'xxl');

create table products (
  id serial primary key,
  name text not null,
  category product_category not null,
  price numeric(10,2) check (price > 0),
  discount_percentage percentage default 0.00,
  rating rating,
  color_code color_hex,
  weight_kg numeric(8,3) check (weight_kg > 0 and weight_kg <= 1000),
  size size_category,
  sku slug unique not null,
  stock_count positive_integer default 0,
  min_age age_range,
  max_temperature temperature_celsius,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

---- create above / drop below ----
drop table products;
drop type size_category;
drop type product_category;
