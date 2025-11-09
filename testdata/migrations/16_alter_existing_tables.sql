-- 16_alter_existing_tables.sql
alter table posts add column priority priority_level default 'low';
alter table posts add column post_slug slug;

alter table comments add column reply_to_comment_id integer references comments(id);

---- create above / drop below ----
alter table comments drop column reply_to_comment_id;
alter table posts drop column post_slug;
alter table posts drop column priority;
