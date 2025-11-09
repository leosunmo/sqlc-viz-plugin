-- 21_create_location_tracking.sql
create table locations (
  id serial primary key,
  name text not null,
  latitude latitude not null,
  longitude longitude not null,
  address text,
  created_at timestamptz default now()
);

create table weather_readings (
  id serial primary key,
  location_id integer references locations(id),
  temperature temperature_celsius not null,
  humidity percentage not null,
  recorded_at timestamptz default now(),
  constraint valid_weather_reading check (
    (temperature >= -50 and humidity >= 0) or 
    (temperature <= 50 and humidity <= 100)
  )
);

---- create above / drop below ----
drop table weather_readings;
drop table locations;
