-- 10_alter_comments_add_email.sql
alter table comments add column author_email email_address;

---- create above / drop below ----
alter table comments drop column author_email;
