-- 11_create_composite_type.sql
create type address as (
  street text,
  city text,
  zipcode text,
  country text
);

create type contact_info as (
  phone text,
  email email_address,
  preferred_contact_method text
);

---- create above / drop below ----
drop type contact_info;
drop type address;
