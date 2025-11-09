-- 12_create_user_profiles.sql
create table user_profiles (
  id serial primary key,
  user_id integer references users(id),
  full_address address,
  contact contact_info,
  current_mood mood default 'neutral',
  created_at timestamptz default now()
);

---- create above / drop below ----
drop table user_profiles;
