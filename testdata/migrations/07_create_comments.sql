-- 07_create_comments.sql
create table comments (
  id serial primary key,
  post_id integer references posts(id),
  author text not null,
  body text,
  created_at timestamptz default now()
);

---- create above / drop below ----
drop table comments;
