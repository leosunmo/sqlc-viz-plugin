-- 14_create_tasks_table.sql
create table tasks (
  id serial primary key,
  user_id integer references users(id),
  title text not null,
  description text,
  priority priority_level default 'medium',
  estimated_hours positive_integer,
  slug slug unique,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

---- create above / drop below ----
drop table tasks;
