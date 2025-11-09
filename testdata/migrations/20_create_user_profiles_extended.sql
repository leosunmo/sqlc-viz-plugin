-- 20_create_user_profiles_extended.sql
create table user_profiles_extended (
  id serial primary key,
  user_id integer references users(id) unique,
  phone phone_number,
  age age_range,
  credit_score credit_score,
  bio short_text,
  website_slug slug,
  completion_percentage percentage default 0.00,
  birth_year year_range,
  account_status valid_status default 'pending',
  preferred_contact email_address,
  rating rating,
  theme_color color_hex default '#007bff',
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

---- create above / drop below ----
drop table user_profiles_extended;
