-- 17_create_project_management.sql
create table projects (
  id serial primary key,
  name short_text not null,
  description text,
  completion_percentage percentage default 0.00,
  status valid_status default 'active',
  start_year year_range,
  created_by integer references users(id),
  created_at timestamptz default now()
);

create table project_members (
  id serial primary key,
  project_id integer references projects(id),
  user_id integer references users(id),
  role short_text default 'member',
  join_year year_range,
  unique(project_id, user_id)
);

---- create above / drop below ----
drop table project_members;
drop table projects;