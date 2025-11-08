-- 08_create_comment_view.sql
create view post_comment_count as
select p.id as post_id, p.title, count(c.id) as comment_count
from posts p
left join comments c on p.id = c.post_id
group by p.id, p.title;

---- create above / drop below ----
drop view post_comment_count;
