-- 24_create_advanced_constraints.sql
create domain business_hours as integer check (
  not (value < 8 or value > 18) and value is not null
);

create domain secure_password as text check (
  length(value) >= 8 and 
  value ~ '[A-Z]' and 
  value ~ '[a-z]' and 
  value ~ '[0-9]' and
  value ~ '[!@#$%^&*()]'
);

create domain ip_address as text check (
  value ~ '^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$'
);

create table security_settings (
  id serial primary key,
  user_id integer references users(id) unique,
  password_hash secure_password not null,
  allowed_ip ip_address,
  login_hours business_hours default 9,
  session_timeout_minutes positive_integer default 30,
  two_factor_enabled boolean default false,
  created_at timestamptz default now(),
  constraint security_logic check (
    (two_factor_enabled = true and session_timeout_minutes <= 60) or
    (two_factor_enabled = false and session_timeout_minutes <= 120)
  )
);

---- create above / drop below ----
drop table security_settings;
drop domain ip_address;
drop domain secure_password;
drop domain business_hours;
