-- 05_alter_posts_add_column.sql
alter table posts add column is_published boolean default false;

---- create above / drop below ----
alter table posts drop column is_published;
