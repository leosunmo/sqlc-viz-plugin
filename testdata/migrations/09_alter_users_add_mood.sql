-- 09_alter_users_add_mood.sql
alter table users add column current_mood mood;

---- create above / drop below ----
alter table users drop column current_mood;
