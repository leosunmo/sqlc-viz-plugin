-- 04_create_post_view.sql
create view user_post_summary as
select u.username, p.title, p.mood, p.posted_at
from users u
join posts p on u.id = p.user_id;

---- create above / drop below ----
drop view user_post_summary;
