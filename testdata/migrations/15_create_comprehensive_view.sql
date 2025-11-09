-- 15_create_comprehensive_view.sql
create view user_activity_summary as
select 
  u.id as user_id,
  u.username,
  u.email,
  u.current_mood,
  up.full_address,
  up.contact,
  count(distinct p.id) as post_count,
  count(distinct c.id) as comment_count,
  count(distinct t.id) as task_count,
  max(p.posted_at) as last_post_date,
  max(c.created_at) as last_comment_date
from users u
left join user_profiles up on u.id = up.user_id
left join posts p on u.id = p.user_id
left join comments c on u.id = (select user_id from posts where id = c.post_id)
left join tasks t on u.id = t.user_id
group by u.id, u.username, u.email, u.current_mood, up.full_address, up.contact;

---- create above / drop below ----
drop view user_activity_summary;
