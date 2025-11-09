-- 18_create_advanced_domains.sql
create domain phone_number as text check (value ~* '^\+?[1-9]\d{1,14}$');
create domain rating as numeric(2,1) check (value between 1.0 and 5.0);
create domain age_range as integer check (value >= 13 and value <= 120);
create domain color_hex as text check (value ~* '^#[0-9A-Fa-f]{6}$');
create domain credit_score as integer check (value between 300 and 850);
create domain latitude as decimal(10,8) check (value between -90.0 and 90.0);
create domain longitude as decimal(11,8) check (value between -180.0 and 180.0);
create domain temperature_celsius as numeric(5,2) check (value between -273.15 and 1000.0);

---- create above / drop below ----
drop domain temperature_celsius;
drop domain longitude;
drop domain latitude;
drop domain credit_score;
drop domain color_hex;
drop domain age_range;
drop domain rating;
drop domain phone_number;
