-- 02_create_custom_types.sql
create type mood as enum ('happy', 'sad', 'neutral');
create domain email_address as text check (value ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

---- create above / drop below ----
drop domain email_address;
drop type mood;
