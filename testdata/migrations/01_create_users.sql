-- 01_create_users.sql
create table users (
  id serial primary key,
  username text not null unique,
  email text not null unique,
  created_at timestamptz default now()
);

---- create above / drop below ----

drop table users;
