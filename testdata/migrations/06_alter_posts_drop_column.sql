drop view if exists user_post_summary;
alter table posts drop column mood;

---- create above / drop below ----
alter table posts add column mood mood;
create view user_post_summary as
select u.username, p.title, p.mood, p.posted_at
from users u
join posts p on u.id = p.user_id;
