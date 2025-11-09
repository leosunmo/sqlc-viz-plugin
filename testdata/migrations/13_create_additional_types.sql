-- 13_create_additional_types.sql
create type priority_level as enum ('low', 'medium', 'high', 'urgent');
create domain positive_integer as integer check (value > 0);
create domain slug as text check (value ~* '^[a-z0-9]+(?:-[a-z0-9]+)*$');
create domain percentage as numeric(5,2) check (value >= 0.00 and value <= 100.00);
create domain short_text as varchar(50) check (length(value) >= 3);
create domain valid_status as text check (value in ('active', 'inactive', 'pending', 'suspended'));
create domain year_range as integer check (value between 1900 and 2100);

---- create above / drop below ----
drop domain year_range;
drop domain valid_status;
drop domain short_text;
drop domain percentage;
drop domain slug;
drop domain positive_integer;
drop type priority_level;
