-- 03_create_posts.sql
create table posts (
  id serial primary key,
  user_id integer references users(id),
  title text not null,
  body text,
  mood mood,
  posted_at timestamptz default now()
);

---- create above / drop below ----
drop table posts;
